// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';
import {useSelector} from 'react-redux';

import {getCurrentTeam} from 'mattermost-redux/selectors/entities/teams';

import SidebarBrowseOrAddChannelMenu from './sidebar_browse_or_add_channel_menu';
import SidebarTeamMenu from './sidebar_team_menu';
import ChannelNavigator from '../channel_navigator';
import {localizeMessage} from 'utils/utils';

import chatSidebarHeaderLogo from 'images/chat-sidebar-header-logo.png';

import './sidebar_header.scss';

import Search from 'components/search/index';

export type Props = {
    showNewChannelModal: () => void;
    showMoreChannelsModal: () => void;
    showCreateUserGroupModal: () => void;
    invitePeopleModal: () => void;
    showCreateCategoryModal: () => void;
    canCreateChannel: boolean;
    canJoinPublicChannel: boolean;
    handleOpenDirectMessagesModal: () => void;
    unreadFilterEnabled: boolean;
    canCreateCustomGroups: boolean;
}

const getChatLogo = () => {
        return (
            <img
                className='sidebar-header-chat-logo'
                alt='chat'
                src={chatSidebarHeaderLogo}
            />
        );
};



const SidebarHeader = (props: Props) => {
    const currentTeam = useSelector(getCurrentTeam);
    const ariaLabel = localizeMessage({id: 'accessibility.sections.lhsNavigator', defaultMessage: 'channel navigator region'});
    
    if (!currentTeam) {
        return null;
    }

    return (
        <div className='sidebarHeaderContainer'>
            <div className='sidebarHeaderTitleContainer'>
                {getChatLogo()}
                {/* <SidebarTeamMenu currentTeam={currentTeam}/> */}
                {(props.canCreateChannel || props.canJoinPublicChannel) && (
                    <SidebarBrowseOrAddChannelMenu
                        canCreateChannel={props.canCreateChannel}
                        onCreateNewChannelClick={props.showNewChannelModal}
                        canJoinPublicChannel={props.canJoinPublicChannel}
                        onBrowseChannelClick={props.showMoreChannelsModal}
                        onOpenDirectMessageClick={props.handleOpenDirectMessagesModal}
                        canCreateCustomGroups={props.canCreateCustomGroups}
                        onCreateNewUserGroupClick={props.showCreateUserGroupModal}
                        unreadFilterEnabled={props.unreadFilterEnabled}
                        onCreateNewCategoryClick={props.showCreateCategoryModal}
                        onInvitePeopleClick={props.invitePeopleModal}
                    />
                    
                )}
            </div>
            <div className='sidebarHeaderNavigationContainer'>
                <div
                    id='rhsNavigator'
                    role='application'
                    aria-label={ariaLabel}
                    className='a11y__region'
                    data-a11y-sort-order='6'
                >
                {/* <ChannelNavigator/> */}
                <Search
                    isSideBarRight={false}
                    isSideBarRightOpen={false}
                >
                    
                </Search>
                </div>
            </div>
        </div>
    );
};

export default SidebarHeader;
