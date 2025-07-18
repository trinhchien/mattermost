// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import type {SidebarSize} from 'components/resizable_sidebar/constants';

export enum ChannelFilterType {
    ALL = 'all',
    GROUP = 'channels',
    DIRECT = 'direct_messages',
};

export type LhsViewState = {
    isOpen: boolean;

    size: SidebarSize;

    // Static pages (e.g. Threads, Insights, etc.)
    currentStaticPageId: string;
    currentChannelFilter: ChannelFilterType
}

export enum LhsItemType {
    None = 'none',
    Page = 'page',
    Channel = 'channel',
}

export enum LhsPage {
    Drafts = 'drafts',
    Threads = 'threads',
}

export type StaticPage = {
    id: string;
    isVisible: boolean;
}


