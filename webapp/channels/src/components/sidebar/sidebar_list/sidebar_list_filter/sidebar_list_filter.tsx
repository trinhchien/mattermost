import { setChannelFilterType } from 'actions/views/lhs';
import React from 'react'
import {ChannelFilterType} from 'types/store/lhs';

const ChannelFilterButtons = ({handleClick, selectedBtn}: {handleClick: (filterType: ChannelFilterType) => void, selectedBtn: ChannelFilterType}) => {

    return (
        <div style={{display: 'flex', gap: '8px'}}>
            <button
                className={selectedBtn === ChannelFilterType.ALL ? 'selected' : ''}
                onClick={() => handleClick(ChannelFilterType.ALL)}
            >
                Tất cả
            </button>
            <button
                className={selectedBtn === ChannelFilterType.GROUP ? 'selected' : ''}
                onClick={() => handleClick(ChannelFilterType.GROUP)}
            >
                Nhóm
            </button>
            <button
                className={selectedBtn === ChannelFilterType.DIRECT ? 'selected' : ''}
                onClick={() => handleClick(ChannelFilterType.DIRECT)}
            >
                Tin nhắn trực tiếp
            </button>

            <style>{`
                button {
                    padding: 8px 12px;
                    border: 1px solid #ccc;
                    background: white;
                    border-radius: 4px;
                    cursor: pointer;
                }
                button.selected {
                    background: #007bff;
                    color: white;
                    border-color: #007bff;
                }
            `}</style>
        </div>
    );
};

export default ChannelFilterButtons;