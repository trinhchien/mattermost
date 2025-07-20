import { setChannelFilterType } from 'actions/views/lhs';
import React from 'react'
import {ChannelFilterType} from 'types/store/lhs';
import './sidebar_list_filter.scss';

const ChannelFilterButtons = ({handleClick, selectedBtn, channelCount}: {handleClick: (filterType: ChannelFilterType) => void, selectedBtn: ChannelFilterType, channelCount: number}) => {

    return (
        <div className = 'filter-btn-container'>
            <button
                className={`filter-btn ${selectedBtn === ChannelFilterType.ALL ? 'selected' : ''}`} 
                onClick={() => handleClick(ChannelFilterType.ALL)}
            >
                <span className="filter-btn-label">Tất cả</span>
                {selectedBtn === ChannelFilterType.ALL && (
                    <span className={`filter-btn-count ${selectedBtn === ChannelFilterType.ALL ? 'selected' : ''}`}>
                        {channelCount}
                    </span>
                )}
            </button>
            <button
                className={`filter-btn ${selectedBtn === ChannelFilterType.GROUP ? 'selected' : ''}`} 
                onClick={() => handleClick(ChannelFilterType.GROUP)}
            >
                <span className="filter-btn-label">Nhóm</span>
                {selectedBtn === ChannelFilterType.GROUP && (
                    <span className={`filter-btn-count ${selectedBtn === ChannelFilterType.GROUP ? 'selected' : ''}`}>
                        {channelCount}
                    </span>
                )}
            </button>
            <button
                className={`filter-btn ${selectedBtn === ChannelFilterType.DIRECT ? 'selected' : ''}`} 
                onClick={() => handleClick(ChannelFilterType.DIRECT)}
            >
                <span className="filter-btn-label">Cá nhân</span>
                {selectedBtn === ChannelFilterType.DIRECT && (
                    <span className={`filter-btn-count ${selectedBtn === ChannelFilterType.DIRECT ? 'selected' : ''}`}>
                        {channelCount}
                    </span>
                )}
            </button>
        </div>
    );
};

export default ChannelFilterButtons;