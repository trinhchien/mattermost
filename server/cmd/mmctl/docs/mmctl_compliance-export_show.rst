.. _mmctl_compliance-export_show:

mmctl compliance-export show
----------------------------

Show compliance export job

Synopsis
~~~~~~~~


Show compliance export job

::

  mmctl compliance-export show [complianceExportJobID] [flags]

Examples
~~~~~~~~

::

  compliance-export show o98rj3ur83dp5dppfyk5yk6osy

Options
~~~~~~~

::

  -h, --help   help for show

Options inherited from parent commands
~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~

::

      --config string                path to the configuration file (default "$XDG_CONFIG_HOME/mmctl/config")
      --disable-pager                disables paged output
      --insecure-sha1-intermediate   allows to use insecure TLS protocols, such as SHA-1
      --insecure-tls-version         allows to use TLS versions 1.0 and 1.1
      --json                         the output format will be in json format
      --local                        allows communicating with the server through a unix socket
      --quiet                        prevent mmctl to generate output for the commands
      --strict                       will only run commands if the mmctl version matches the server one
      --suppress-warnings            disables printing warning messages

SEE ALSO
~~~~~~~~

* `mmctl compliance-export <mmctl_compliance-export.rst>`_ 	 - Management of compliance exports

