// src/components/ValidatorList.js

import React from 'react';

function ValidatorList({from, to, validators, sortConfig, setSortConfig}) {
    const sortValidators = (column) => {
        let direction = 'asc';
        if (sortConfig.column === column && sortConfig.direction === 'asc') {
            direction = 'desc';
        }
        setSortConfig({column, direction});
    };

    const sortedValidators = [...validators].sort((a, b) => {
        const {column, direction} = sortConfig;
        if (column === 'stake') {
            return direction === 'asc' ? a.stake - b.stake : b.stake - a.stake;
        } else if (column === 'index') {
            return direction === 'asc' ? a.index - b.index : b.index - a.index;
        } else if (column === 'avgEfficiency') {
            return direction === 'asc' ? a.avgEfficiency - b.avgEfficiency : b.avgEfficiency - a.avgEfficiency;
        } else {
            return direction === 'asc'
                ? a.adnl.localeCompare(b.adnl)
                : b.adnl.localeCompare(a.adnl);
        }
    });

    const formatNumberToText = (num) => {
        if (Math.abs(num) >= 1.0e9) {
            return (num / 1.0e9).toFixed(1) + 'B';
        } else if (Math.abs(num) >= 1.0e6) {
            return (num / 1.0e6).toFixed(1) + 'M';
        } else if (Math.abs(num) >= 1.0e3) {
            return (num / 1.0e3).toFixed(1) + 'K';
        } else {
            return num.toString();
        }
    };

    return (
        <div id="validator-list">
            <h2>Validators</h2>
            <table className="validator-table">
                <thead>
                <tr>
                    <th onClick={() => sortValidators('index')}>#</th>
                    <th onClick={() => sortValidators('stake')}>Stake</th>
                    <th onClick={() => sortValidators('adnl')}>ADNL</th>
                    <th>Status</th>
                    <th onClick={() => sortValidators('avgEfficiency')}>Eff.</th>
                </tr>
                </thead>
                <tbody>
                {sortedValidators.map((validator) => (
                    <tr key={validator.adnl}>
                        <td>{validator.index}</td>
                        <td>
                            <span className="validator__stake">{formatNumberToText(validator.stake)}</span>
                            <span className="validator__weight">{formatNumberToText(validator.weight)}</span>

                        </td>
                        <td>
                            <a className="validator__adnl"
                               href={`?adnl=${encodeURIComponent(validator.adnl)}&from=${from}&to=${to}`}
                            >
                                {validator.adnl}
                            </a>
                            <a className="validator__wallet" target="_blank"
                               href={`https://tonscan.org/single-nominator/${validator.walletAddress}`}
                            >{validator.walletAddress}</a>
                        </td>
                        <td>
                            <div className="status-bars">
                                {Array.from({length: 60}).map((_, index) => {
                                    const statusItem =
                                        validator.statuses && validator.statuses[index];
                                    return (
                                        <div
                                            key={index}
                                            className={`status-bar ${
                                                !statusItem ||
                                                statusItem.status === null ||
                                                statusItem.status === 0
                                                    ? 'no-data'
                                                    : statusItem.status < 80
                                                        ? 'bad'
                                                        : ''
                                            }`}
                                            data-tooltip={
                                                statusItem
                                                    ? `${
                                                        statusItem.status !== null
                                                            ? statusItem.status.toFixed(2)
                                                            : 'No data'
                                                    }% (${new Date(
                                                        statusItem.timestamp * 1000
                                                    ).toLocaleString()})`
                                                    : 'No data'
                                            }
                                        ></div>
                                    );
                                })}
                            </div>
                        </td>
                        <td>
                            <span
                                className={validator.avgEfficiency < 80 ? 'validator__efficiency error' : 'validator__efficiency ok'}>{validator.avgEfficiency}%</span>
                        </td>
                    </tr>
                ))}
                </tbody>
            </table>
        </div>
    );
}

export default ValidatorList;
