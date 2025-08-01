// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package sqlstore

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/lib/pq"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin/plugintest/mock"
	"github.com/mattermost/mattermost/server/public/shared/mlog"
	"github.com/mattermost/mattermost/server/public/shared/request"
	"github.com/mattermost/mattermost/server/v8/channels/db"
	"github.com/mattermost/mattermost/server/v8/channels/store"
	"github.com/mattermost/mattermost/server/v8/channels/store/searchtest"
	"github.com/mattermost/mattermost/server/v8/channels/store/storetest"
	"github.com/mattermost/mattermost/server/v8/einterfaces/mocks"
)

type storeType struct {
	Name        string
	SqlSettings *model.SqlSettings
	SqlStore    *SqlStore
	Store       store.Store
}

var (
	storeTypes []*storeType
	storePools []*TestPool
)

func getStoresFromPools(t *testing.T) []*storeType {
	stores := make([]*storeType, len(storePools))
	for i, pool := range storePools {
		entry := pool.Get(t)
		if entry == nil {
			panic("failed to get store from pool")
		}

		entry.Store.DropAllTables()
		entry.Store.MarkSystemRanUnitTests()

		stores[i] = &storeType{
			Name:        *entry.Settings.DriverName,
			SqlSettings: entry.Settings,
			SqlStore:    entry.Store,
			Store:       entry.Store,
		}
	}

	return stores
}

func newStoreType(name, driver string) *storeType {
	return &storeType{
		Name:        name,
		SqlSettings: storetest.MakeSqlSettings(driver),
	}
}

func StoreTest(t *testing.T, f func(*testing.T, request.CTX, store.Store)) {
	defer func() {
		if err := recover(); err != nil {
			tearDownStores()
			panic(err)
		}
	}()

	stores := storeTypes
	if enableFullyParallelTests {
		t.Parallel()
		stores = getStoresFromPools(t)
	}

	for _, st := range stores {
		rctx := request.TestContext(t)

		t.Run(st.Name, func(t *testing.T) {
			if testing.Short() {
				t.SkipNow()
			}

			f(t, rctx, st.Store)
		})
	}
}

func StoreTestWithSearchTestEngine(t *testing.T, f func(*testing.T, store.Store, *searchtest.SearchTestEngine)) {
	defer func() {
		if err := recover(); err != nil {
			tearDownStores()
			panic(err)
		}
	}()

	stores := storeTypes
	if enableFullyParallelTests {
		t.Parallel()
		stores = getStoresFromPools(t)
	}

	for _, st := range stores {
		searchTestEngine := &searchtest.SearchTestEngine{
			Driver: *st.SqlSettings.DriverName,
		}

		t.Run(st.Name, func(t *testing.T) {
			f(t, st.Store, searchTestEngine)
		})
	}
}

func StoreTestWithSqlStore(t *testing.T, f func(*testing.T, request.CTX, store.Store, storetest.SqlStore)) {
	defer func() {
		if err := recover(); err != nil {
			tearDownStores()
			panic(err)
		}
	}()

	stores := storeTypes
	if enableFullyParallelTests {
		t.Parallel()
		stores = getStoresFromPools(t)
	}

	for _, st := range stores {
		rctx := request.TestContext(t)

		t.Run(st.Name, func(t *testing.T) {
			if testing.Short() {
				t.SkipNow()
			}
			f(t, rctx, st.Store, &StoreTestWrapper{st.SqlStore})
		})
	}
}

func initStores(logger mlog.LoggerIFace, parallelism int) {
	if testing.Short() {
		return
	}

	// NOTE: we use a pool size higher than the parallelism value (coming from -test.parallel flag) as we need a bit of extra buffer to cover
	// for subtests or paused tests that might also run in parallel and initialize a new store.
	parallelTestsPoolSize := parallelism * 2

	// In CI, we run the test suite for the current database set.
	if os.Getenv("IS_CI") == "true" {
		switch os.Getenv("MM_SQLSETTINGS_DRIVERNAME") {
		case "postgres":
			storeTypes = append(storeTypes, newStoreType("PostgreSQL", model.DatabaseDriverPostgres))
			if enableFullyParallelTests {
				pgStorePool, err := NewTestPool(logger, model.DatabaseDriverPostgres, parallelTestsPoolSize)
				if err != nil {
					panic(err)
				}
				storePools = append(storePools, pgStorePool)
			}
		}
	} else {
		storeTypes = append(storeTypes,
			newStoreType("PostgreSQL", model.DatabaseDriverPostgres),
		)

		if enableFullyParallelTests {
			pgStorePool, err := NewTestPool(logger, model.DatabaseDriverPostgres, parallelTestsPoolSize)
			if err != nil {
				panic(err)
			}
			storePools = append(storePools, pgStorePool)
		}
	}

	defer func() {
		if err := recover(); err != nil {
			tearDownStores()
			panic(err)
		}
	}()

	var eg errgroup.Group
	for _, st := range storeTypes {
		eg.Go(func() error {
			var err error
			st.SqlStore, err = New(*st.SqlSettings, logger, nil)
			if err != nil {
				return err
			}
			st.Store = st.SqlStore
			st.Store.DropAllTables()
			st.Store.MarkSystemRanUnitTests()

			return nil
		})
	}
	if err := eg.Wait(); err != nil {
		panic(err)
	}
}

var tearDownStoresOnce sync.Once

func tearDownStores() {
	if testing.Short() {
		return
	}
	tearDownStoresOnce.Do(func() {
		var wg sync.WaitGroup
		wg.Add(len(storeTypes))
		for _, st := range storeTypes {
			go func() {
				if st.Store != nil {
					st.Store.Close()
				}
				if st.SqlSettings != nil {
					storetest.CleanupSqlSettings(st.SqlSettings)
				}
				wg.Done()
			}()
		}

		var wgPool sync.WaitGroup
		wgPool.Add(len(storePools))
		for _, pool := range storePools {
			go func() {
				defer wgPool.Done()
				pool.Close()
			}()
		}

		wg.Wait()
		wgPool.Wait()
	})
}

// This test was used to consistently reproduce the race
// before the fix in MM-28397.
// Keeping it here to help avoiding future regressions.
func TestStoreLicenseRace(t *testing.T) {
	if enableFullyParallelTests {
		t.Parallel()
	}

	logger := mlog.CreateTestLogger(t)

	settings, err := makeSqlSettings(model.DatabaseDriverPostgres)
	if err != nil {
		t.Skip(err)
	}

	store, err := New(*settings, logger, nil)
	require.NoError(t, err)
	defer func() {
		store.Close()
		storetest.CleanupSqlSettings(settings)
	}()

	wg := sync.WaitGroup{}
	wg.Add(3)

	go func() {
		store.UpdateLicense(&model.License{})
		wg.Done()
	}()

	go func() {
		store.GetReplica()
		wg.Done()
	}()

	go func() {
		store.GetSearchReplicaX()
		wg.Done()
	}()

	wg.Wait()
}

func TestGetReplica(t *testing.T) {
	t.Parallel()

	logger := mlog.CreateTestLogger(t)

	testCases := []struct {
		Description                string
		DataSourceReplicaNum       int
		DataSourceSearchReplicaNum int
	}{
		{
			"no replicas",
			0,
			0,
		},
		{
			"one source replica",
			1,
			0,
		},
		{
			"multiple source replicas",
			3,
			0,
		},
		{
			"one source search replica",
			0,
			1,
		},
		{
			"multiple source search replicas",
			0,
			3,
		},
		{
			"one source replica, one source search replica",
			1,
			1,
		},
		{
			"one source replica, multiple source search replicas",
			1,
			3,
		},
		{
			"multiple source replica, one source search replica",
			3,
			1,
		},
		{
			"multiple source replica, multiple source search replicas",
			3,
			3,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.Description+" with license", func(t *testing.T) {
			settings, err := makeSqlSettings(model.DatabaseDriverPostgres)
			if err != nil {
				t.Skip(err)
			}

			dataSourceReplicas := []string{}
			dataSourceSearchReplicas := []string{}
			for i := 0; i < testCase.DataSourceReplicaNum; i++ {
				dataSourceReplicas = append(dataSourceReplicas, *settings.DataSource)
			}
			for i := 0; i < testCase.DataSourceSearchReplicaNum; i++ {
				dataSourceSearchReplicas = append(dataSourceSearchReplicas, *settings.DataSource)
			}

			settings.DataSourceReplicas = dataSourceReplicas
			settings.DataSourceSearchReplicas = dataSourceSearchReplicas
			store, err := New(*settings, logger, nil)
			require.NoError(t, err)
			defer func() {
				store.Close()
				storetest.CleanupSqlSettings(settings)
			}()

			store.UpdateLicense(&model.License{})

			replicas := make(map[*sqlxDBWrapper]bool)
			for range 5 {
				replicas[store.GetReplica()] = true
			}

			searchReplicas := make(map[*sqlxDBWrapper]bool)
			for range 5 {
				searchReplicas[store.GetSearchReplicaX()] = true
			}

			if testCase.DataSourceReplicaNum > 0 {
				// If replicas were defined, ensure none are the master.
				assert.Len(t, replicas, testCase.DataSourceReplicaNum)

				for replica := range replicas {
					assert.NotSame(t, store.GetMaster(), replica)
				}
			} else if assert.Len(t, replicas, 1) {
				// Otherwise ensure the replicas contains only the master.
				for replica := range replicas {
					assert.Same(t, store.GetMaster(), replica)
				}
			}

			if testCase.DataSourceSearchReplicaNum > 0 {
				// If search replicas were defined, ensure none are the master nor the replicas.
				assert.Len(t, searchReplicas, testCase.DataSourceSearchReplicaNum)

				for searchReplica := range searchReplicas {
					assert.NotSame(t, store.GetMaster(), searchReplica)
					for replica := range replicas {
						assert.NotSame(t, searchReplica, replica)
					}
				}
			} else if testCase.DataSourceReplicaNum > 0 {
				assert.Equal(t, len(replicas), len(searchReplicas))
				for k := range replicas {
					assert.True(t, searchReplicas[k])
				}
			} else if testCase.DataSourceReplicaNum == 0 && assert.Len(t, searchReplicas, 1) {
				// Otherwise ensure the search replicas contains the master.
				for searchReplica := range searchReplicas {
					assert.Same(t, store.GetMaster(), searchReplica)
				}
			}
		})

		t.Run(testCase.Description+" without license", func(t *testing.T) {
			settings, err := makeSqlSettings(model.DatabaseDriverPostgres)
			if err != nil {
				t.Skip(err)
			}

			dataSourceReplicas := []string{}
			dataSourceSearchReplicas := []string{}
			for i := 0; i < testCase.DataSourceReplicaNum; i++ {
				dataSourceReplicas = append(dataSourceReplicas, *settings.DataSource)
			}
			for i := 0; i < testCase.DataSourceSearchReplicaNum; i++ {
				dataSourceSearchReplicas = append(dataSourceSearchReplicas, *settings.DataSource)
			}

			settings.DataSourceReplicas = dataSourceReplicas
			settings.DataSourceSearchReplicas = dataSourceSearchReplicas
			store, err := New(*settings, logger, nil)
			require.NoError(t, err)
			defer func() {
				store.Close()
				storetest.CleanupSqlSettings(settings)
			}()

			replicas := make(map[*sqlxDBWrapper]bool)
			for range 5 {
				replicas[store.GetReplica()] = true
			}

			searchReplicas := make(map[*sqlxDBWrapper]bool)
			for range 5 {
				searchReplicas[store.GetSearchReplicaX()] = true
			}

			if testCase.DataSourceReplicaNum > 0 {
				// If replicas were defined, ensure none are the master.
				assert.Len(t, replicas, 1)

				for replica := range replicas {
					assert.Same(t, store.GetMaster(), replica)
				}
			} else if assert.Len(t, replicas, 1) {
				// Otherwise ensure the replicas contains only the master.
				for replica := range replicas {
					assert.Same(t, store.GetMaster(), replica)
				}
			}

			if testCase.DataSourceSearchReplicaNum > 0 {
				// If search replicas were defined, ensure none are the master nor the replicas.
				assert.Len(t, searchReplicas, 1)

				for searchReplica := range searchReplicas {
					assert.Same(t, store.GetMaster(), searchReplica)
				}
			} else if testCase.DataSourceReplicaNum > 0 {
				assert.Equal(t, len(replicas), len(searchReplicas))
				for k := range replicas {
					assert.True(t, searchReplicas[k])
				}
			} else if assert.Len(t, searchReplicas, 1) {
				// Otherwise ensure the search replicas contains the master.
				for searchReplica := range searchReplicas {
					assert.Same(t, store.GetMaster(), searchReplica)
				}
			}
		})
	}
}

func TestGetDbVersion(t *testing.T) {
	if enableFullyParallelTests {
		t.Parallel()
	}

	logger := mlog.CreateTestLogger(t)

	testDrivers := []string{
		model.DatabaseDriverPostgres,
	}

	for _, d := range testDrivers {
		driver := d
		t.Run("Should return db version for "+driver, func(t *testing.T) {
			t.Parallel()
			settings, err := makeSqlSettings(driver)
			if err != nil {
				t.Skip(err)
			}

			store, err := New(*settings, logger, nil)
			require.NoError(t, err)

			version, err := store.GetDbVersion(false)
			require.NoError(t, err)
			require.Regexp(t, regexp.MustCompile(`\d+\.\d+(\.\d+)?`), version)
		})
	}
}

func TestEnsureMinimumDBVersion(t *testing.T) {
	if enableFullyParallelTests {
		t.Parallel()
	}

	tests := []struct {
		driver string
		ver    string
		ok     bool
		err    string
	}{
		{
			driver: model.DatabaseDriverPostgres,
			ver:    "110001",
			ok:     false,
			err:    "",
		},
		{
			driver: model.DatabaseDriverPostgres,
			ver:    "130001",
			ok:     true,
			err:    "",
		},
		{
			driver: model.DatabaseDriverPostgres,
			ver:    "90603",
			ok:     false,
			err:    "minimum Postgres version requirements not met",
		},
		{
			driver: model.DatabaseDriverPostgres,
			ver:    "12.34.1",
			ok:     false,
			err:    "cannot parse DB version",
		},
	}

	pg := model.DatabaseDriverPostgres
	pgSettings := &model.SqlSettings{
		DriverName: &pg,
	}
	for _, tc := range tests {
		store := &SqlStore{}
		store.settings = pgSettings
		ok, err := store.ensureMinimumDBVersion(tc.ver)
		assert.Equal(t, tc.ok, ok, "driver: %s, version: %s", tc.driver, tc.ver)
		if tc.err != "" {
			assert.Contains(t, err.Error(), tc.err)
		}
	}
}

func TestIsBinaryParamEnabled(t *testing.T) {
	if enableFullyParallelTests {
		t.Parallel()
	}

	tests := []struct {
		store    SqlStore
		expected bool
	}{
		{
			store: SqlStore{
				settings: &model.SqlSettings{
					DriverName: model.NewPointer(model.DatabaseDriverPostgres),
					DataSource: model.NewPointer("postgres://mmuser:mostest@localhost/loadtest?sslmode=disable\u0026binary_parameters=yes"),
				},
			},
			expected: true,
		},
		{
			store: SqlStore{
				settings: &model.SqlSettings{
					DriverName: model.NewPointer(model.DatabaseDriverPostgres),
					DataSource: model.NewPointer("postgres://mmuser:mostest@localhost/loadtest?sslmode=disable&binary_parameters=yes"),
				},
			},
			expected: true,
		},
		{
			store: SqlStore{
				settings: &model.SqlSettings{
					DriverName: model.NewPointer(model.DatabaseDriverPostgres),
					DataSource: model.NewPointer("postgres://mmuser:mostest@localhost/loadtest?sslmode=disable"),
				},
			},
			expected: false,
		},
	}

	for i := range tests {
		ok, err := tests[i].store.computeBinaryParam()
		require.NoError(t, err)
		assert.Equal(t, tests[i].expected, ok)
	}
}

func TestGetAllConns(t *testing.T) {
	t.Parallel()

	logger := mlog.CreateConsoleTestLogger(t)

	testCases := []struct {
		Description                string
		DataSourceReplicaNum       int
		DataSourceSearchReplicaNum int
		ExpectedNumConnections     int
	}{
		{
			"no replicas",
			0,
			0,
			1,
		},
		{
			"one source replica",
			1,
			0,
			2,
		},
		{
			"multiple source replicas",
			3,
			0,
			4,
		},
		{
			"one source search replica",
			0,
			1,
			1,
		},
		{
			"multiple source search replicas",
			0,
			3,
			1,
		},
		{
			"one source replica, one source search replica",
			1,
			1,
			2,
		},
		{
			"one source replica, multiple source search replicas",
			1,
			3,
			2,
		},
		{
			"multiple source replica, one source search replica",
			3,
			1,
			4,
		},
		{
			"multiple source replica, multiple source search replicas",
			3,
			3,
			4,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.Description, func(t *testing.T) {
			t.Parallel()
			settings, err := makeSqlSettings(model.DatabaseDriverPostgres)
			if err != nil {
				t.Skip(err)
			}
			dataSourceReplicas := []string{}
			dataSourceSearchReplicas := []string{}
			for i := 0; i < testCase.DataSourceReplicaNum; i++ {
				dataSourceReplicas = append(dataSourceReplicas, *settings.DataSource)
			}
			for i := 0; i < testCase.DataSourceSearchReplicaNum; i++ {
				dataSourceSearchReplicas = append(dataSourceSearchReplicas, *settings.DataSource)
			}

			settings.DataSourceReplicas = dataSourceReplicas
			settings.DataSourceSearchReplicas = dataSourceSearchReplicas
			store, err := New(*settings, logger, nil)
			require.NoError(t, err)
			defer func() {
				store.Close()
				storetest.CleanupSqlSettings(settings)
			}()

			assert.Len(t, store.GetAllConns(), testCase.ExpectedNumConnections)
		})
	}
}

func TestIsDuplicate(t *testing.T) {
	if enableFullyParallelTests {
		t.Parallel()
	}

	testErrors := map[error]bool{
		&pq.Error{Code: "42P06"}:             false,
		&pq.Error{Code: PGDupTableErrorCode}: true,
		errors.New("Random error"):           false,
	}

	for e, b := range testErrors {
		err := e
		expected := b
		t.Run(fmt.Sprintf("Should return %t for %s", expected, err.Error()), func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, expected, IsDuplicate(err))
		})
	}
}

func TestVersionString(t *testing.T) {
	if enableFullyParallelTests {
		t.Parallel()
	}

	versions := []struct {
		input  int
		driver string
		output string
	}{
		{
			input:  100000,
			driver: model.DatabaseDriverPostgres,
			output: "10.0",
		},
		{
			input:  90603,
			driver: model.DatabaseDriverPostgres,
			output: "9.603",
		},
		{
			input:  120005,
			driver: model.DatabaseDriverPostgres,
			output: "12.5",
		},
	}

	for _, v := range versions {
		out := versionString(v.input, v.driver)
		assert.Equal(t, v.output, out)
	}
}

func TestReplicaLagQuery(t *testing.T) {
	if enableFullyParallelTests {
		t.Parallel()
	}

	testDrivers := []string{
		model.DatabaseDriverPostgres,
	}

	for _, driver := range testDrivers {
		t.Run(driver, func(t *testing.T) {
			settings, err := makeSqlSettings(driver)
			if err != nil {
				t.Skip(err)
			}
			var query string
			var tableName string
			// Just any random query which returns a row in (string, int) format.
			switch driver {
			case model.DatabaseDriverPostgres:
				query = `SELECT relname, count(relname) FROM pg_class WHERE relname='posts' GROUP BY relname`
				tableName = "posts"
			}

			settings.ReplicaLagSettings = []*model.ReplicaLagSettings{{
				DataSource:       model.NewPointer(*settings.DataSource),
				QueryAbsoluteLag: model.NewPointer(query),
				QueryTimeLag:     model.NewPointer(query),
			}}

			mockMetrics := &mocks.MetricsInterface{}
			mockMetrics.On("SetReplicaLagAbsolute", tableName, float64(1))
			mockMetrics.On("SetReplicaLagTime", tableName, float64(1))
			mockMetrics.On("RegisterDBCollector", mock.AnythingOfType("*sql.DB"), "master")

			store := &SqlStore{
				rrCounter:   0,
				srCounter:   0,
				settings:    settings,
				metrics:     mockMetrics,
				logger:      mlog.CreateConsoleTestLogger(t),
				quitMonitor: make(chan struct{}),
				wgMonitor:   &sync.WaitGroup{},
			}

			require.NoError(t, store.initConnection())
			store.stores.post = newSqlPostStore(store, mockMetrics)
			err = store.migrate(migrationsDirectionUp, false, true)
			require.NoError(t, err)

			defer store.Close()

			err = store.ReplicaLagAbs()
			require.NoError(t, err)
			err = store.ReplicaLagTime()
			require.NoError(t, err)
			mockMetrics.AssertExpectations(t)
		})
	}
}

func TestInvalidReplicaLagDataSource(t *testing.T) {
	if enableFullyParallelTests {
		t.Parallel()
	}

	logger := mlog.CreateConsoleTestLogger(t)

	testDrivers := []string{
		model.DatabaseDriverPostgres,
	}

	for _, driver := range testDrivers {
		t.Run(driver, func(t *testing.T) {
			settings, err := makeSqlSettings(driver)
			if err != nil {
				t.Skip(err)
			}

			// Set an invalid DataSource that will fail to connect
			settings.ReplicaLagSettings = []*model.ReplicaLagSettings{{
				DataSource:       model.NewPointer("invalid://connection/string"),
				QueryAbsoluteLag: model.NewPointer("SELECT 1"),
				QueryTimeLag:     model.NewPointer("SELECT 1"),
			}}

			mockMetrics := &mocks.MetricsInterface{}
			mockMetrics.On("RegisterDBCollector", mock.AnythingOfType("*sql.DB"), "master")

			store := &SqlStore{
				rrCounter:   0,
				srCounter:   0,
				settings:    settings,
				metrics:     mockMetrics,
				logger:      logger,
				quitMonitor: make(chan struct{}),
				wgMonitor:   &sync.WaitGroup{},
			}

			require.NoError(t, store.initConnection())
			defer store.Close()

			// Verify no replica lag handles were added despite having ReplicaLagSettings
			assert.Equal(t, 0, len(store.replicaLagHandles))
		})
	}
}

var (
	errDriverMismatch    = errors.New("database drivers mismatch")
	errDriverUnsupported = errors.New("database driver not supported")
)

func makeSqlSettings(driver string) (*model.SqlSettings, error) {
	// When running under CI, only one database engine container is launched
	// so here we must error out if the requested driver doesn't match.
	if os.Getenv("IS_CI") == "true" {
		envDriver := os.Getenv("MM_SQLSETTINGS_DRIVERNAME")
		if envDriver != "" && envDriver != driver {
			return nil, errDriverMismatch
		}
	}

	switch driver {
	case model.DatabaseDriverPostgres:
		return storetest.MakeSqlSettings(driver), nil
	}

	return nil, errDriverUnsupported
}

func TestExecNoTimeout(t *testing.T) {
	StoreTest(t, func(t *testing.T, rctx request.CTX, ss store.Store) {
		sqlStore := ss.(*SqlStore)
		timeout := sqlStore.masterX.queryTimeout
		sqlStore.masterX.queryTimeout = 1
		defer func() {
			sqlStore.masterX.queryTimeout = timeout
		}()
		query := `SELECT pg_sleep(2);`
		_, err := sqlStore.GetMaster().ExecNoTimeout(query)
		require.NoError(t, err)
	})
}

func TestGetDBSchemaVersion(t *testing.T) {
	if enableFullyParallelTests {
		t.Parallel()
	}

	testDrivers := []string{
		model.DatabaseDriverPostgres,
	}

	logger := mlog.CreateConsoleTestLogger(t)
	assets := db.Assets()

	for _, d := range testDrivers {
		driver := d
		t.Run("Should return latest version number of applied migrations for "+driver, func(t *testing.T) {
			t.Parallel()
			settings, err := makeSqlSettings(driver)
			if err != nil {
				t.Skip(err)
			}
			store, err := New(*settings, logger, nil)
			require.NoError(t, err)

			assetsList, err := assets.ReadDir(filepath.Join("migrations", driver))
			require.NoError(t, err)

			var assetNamesForDriver []string
			for _, entry := range assetsList {
				assetNamesForDriver = append(assetNamesForDriver, entry.Name())
			}
			sort.Strings(assetNamesForDriver)

			require.NotEmpty(t, assetNamesForDriver)
			lastMigration := assetNamesForDriver[len(assetNamesForDriver)-1]
			expectedVersion := strings.Split(lastMigration, "_")[0]

			version, err := store.GetDBSchemaVersion()
			require.NoError(t, err)
			require.Equal(t, expectedVersion, fmt.Sprintf("%06d", version))
		})
	}
}

func TestGetLocalSchemaVersion(t *testing.T) {
	if enableFullyParallelTests {
		t.Parallel()
	}

	testDrivers := []string{
		model.DatabaseDriverPostgres,
	}

	logger := mlog.CreateConsoleTestLogger(t)

	for _, d := range testDrivers {
		driver := d
		t.Run(driver, func(t *testing.T) {
			settings, err := makeSqlSettings(driver)
			if err != nil {
				t.Skip(err)
			}
			store, err := New(*settings, logger, nil)
			require.NoError(t, err)

			ver, err := store.GetLocalSchemaVersion()
			require.NoError(t, err)

			dbVer, err := store.GetDBSchemaVersion()
			require.NoError(t, err)
			require.Equal(t, ver, dbVer)
		})
	}
}

func TestGetAppliedMigrations(t *testing.T) {
	if enableFullyParallelTests {
		t.Parallel()
	}

	testDrivers := []string{
		model.DatabaseDriverPostgres,
	}

	logger := mlog.CreateConsoleTestLogger(t)
	assets := db.Assets()

	for _, d := range testDrivers {
		driver := d
		t.Run("Should return db applied migrations for "+driver, func(t *testing.T) {
			t.Parallel()
			settings, err := makeSqlSettings(driver)
			if err != nil {
				t.Skip(err)
			}
			store, err := New(*settings, logger, nil)
			require.NoError(t, err)

			assetsList, err := assets.ReadDir(filepath.Join("migrations", driver))
			require.NoError(t, err)

			var migrationsFromFiles []model.AppliedMigration
			for _, entry := range assetsList {
				if strings.HasSuffix(entry.Name(), ".up.sql") {
					versionString := strings.Split(entry.Name(), "_")[0]
					version, vErr := strconv.Atoi(versionString)
					require.NoError(t, vErr)

					name := strings.TrimSuffix(strings.TrimLeft(entry.Name(), versionString+"_"), ".up.sql")

					migrationsFromFiles = append(migrationsFromFiles, model.AppliedMigration{
						Version: version,
						Name:    name,
					})
				}
			}

			require.NotEmpty(t, migrationsFromFiles)

			migrations, err := store.GetAppliedMigrations()
			require.NoError(t, err)
			require.ElementsMatch(t, migrationsFromFiles, migrations)
		})
	}
}

func TestSkipMigrationsOption(t *testing.T) {
	if enableFullyParallelTests {
		t.Parallel()
	}

	testDrivers := []string{
		model.DatabaseDriverPostgres,
	}

	logger := mlog.CreateConsoleTestLogger(t)

	for _, d := range testDrivers {
		driver := d
		t.Run("Should not apply migrations when skipMigrations=true for "+driver, func(t *testing.T) {
			t.Parallel()
			settings, err := makeSqlSettings(driver)
			if err != nil {
				t.Skip(err)
			}

			store, err := New(*settings, logger, nil, SkipMigrations())
			require.NoError(t, err)

			_, err = store.GetDBSchemaVersion()
			assert.Error(t, err)

			// Unfortunately there's no error for table doesn't exist, but we know it won't be ErrNoRows:
			assert.True(t, !errors.Is(err, sql.ErrNoRows))

			// And we know what each db will return:
			assert.Contains(t, err.Error(), "pq: relation \"db_migrations\" does not exist")
		})
	}
}
