import React, {useState} from 'react';
import {useDispatch} from 'react-redux';
import {setChannelViewType} from 'actions/views/channel_sidebar';


export type ButtonType = 'all' | 'group' | 'direct';

interface GroupButtonProps {
    selected: ButtonType;
    onSelect: (type: ButtonType) => void;
}

const buttons: { type: ButtonType; label: string }[] = [
    { type: 'all', label: 'All' },
    { type: 'group', label: 'Group' },
    { type: 'direct', label: 'Direct' },
];

const SideBarListFilter: React.FC<GroupButtonProps> = ({ selected, onSelect }) => {
    return (
        <div >
            <button
                className={selected === 'all' ? 'selected' : ''}
                onClick={() => onSelect('all')}
            >
                Tất cả
            </button>
            <button
                className={selected === 'group' ? 'selected' : ''}
                onClick={() => onSelect('group')}
            >
                Nhóm
            </button>
            <button
                className={selected === 'direct' ? 'selected' : ''}
                onClick={() => onSelect('direct')}
            >
                Tin nhắn trực tiếp
            </button>
        </div>
    );
};

export default SideBarListFilter;