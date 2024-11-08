// src/App.js

import React, {useEffect, useRef, useState} from 'react';
import Header from './components/Header';
import Controls from './components/Controls';
import ValidatorList from './components/ValidatorList';
import EfficiencyChart from './components/EfficiencyChart';
import Timeline from './components/Timeline';
import {useLocation, useNavigate} from 'react-router-dom';

function App() {
    const [efficiencyData, setEfficiencyData] = useState(null);
    const [adnl, setAdnl] = useState('');
    const [from, setFrom] = useState('');
    const [to, setTo] = useState('');
    const [cycleID, setCycleID] = useState('');
    const [validators, setValidators] = useState([]);
    const [sortConfig, setSortConfig] = useState({
        column: 'stake',
        direction: 'desc',
    });

    const location = useLocation();
    const navigate = useNavigate();
    const initialLoad = useRef(true);

    useEffect(() => {
        const params = new URLSearchParams(location.search);
        const adnlParam = params.get('adnl');
        let fromParam = params.get('from');
        let toParam = params.get('to');
        const cycleIDParam = params.get('cycle_id');

        if (!fromParam || !toParam) {
            const toTimestamp = Math.floor(Date.now() / 1000);
            const fromTimestamp = toTimestamp - 60 * 60;
            fromParam = fromTimestamp.toString();
            toParam = toTimestamp.toString();
            setFrom(fromParam);
            setTo(toParam);

            if (initialLoad.current) {
                const newParams = new URLSearchParams(location.search);
                newParams.set('from', fromParam);
                newParams.set('to', toParam);
                navigate({search: `?${newParams.toString()}`}, {replace: true});
            }
        } else {
            setFrom(fromParam);
            setTo(toParam);
        }

        setAdnl(adnlParam || '');
        setCycleID(cycleIDParam || '');

        if (adnlParam && fromParam && toParam) {
            fetchEfficiencyData(adnlParam, fromParam, toParam);
        } else if (fromParam && toParam) {
            fetchValidatorData(fromParam, toParam, cycleIDParam);
        }

        initialLoad.current = false;

        // eslint-disable-next-line
    }, [location.search]);

    const fetchEfficiencyData = async (adnl, from, to) => {
        try {
            const response = await fetch(
                `/api/chart?adnl=${encodeURIComponent(adnl)}&from=${from}&to=${to}`
            );
            const data = await response.json();
            if (data && data[0] && data[0].efficiency) {
                setEfficiencyData(data[0].efficiency);
            } else {
                setEfficiencyData([]);
            }
        } catch (error) {
            console.error('Error fetching efficiency data:', error);
            alert('An error occurred while fetching efficiency data.');
        }
    };

    const fetchValidatorData = async (from, to, cycleID) => {
        if (!from || !to) {
            alert('Please select start and end dates.');
            return;
        }
        if (from > to) {
            alert('"From" date must be earlier than "To" date.');
            return;
        }

        try {
            let url = `/api/validator-statuses?from=${from}&to=${to}`;
            if (cycleID) {
                url += `&cycle_id=${cycleID}`;
            }
            const response = await fetch(url);
            const data = await response.json();

            const processedValidators = processValidatorData(data);
            setValidators(processedValidators);
        } catch (error) {
            console.error('Error fetching validator data:', error);
            alert('An error occurred while fetching validator data.');
        }
    };

    const processValidatorData = (data) => {
        const result = [];

        if (!data || !data.meta || typeof data.meta !== 'object') {
            console.error('Invalid data format: expected data.meta to be an object');
            return result;
        }

        const adnls = Object.keys(data.meta);

        adnls.forEach((adnl) => {
            const validatorMeta = data.meta[adnl];
            const statusesObj = data.statuses && data.statuses[adnl] ? data.statuses[adnl] : {};
            const statusesArray = Object.keys(statusesObj).map((timestamp) => {
                const statusValue = statusesObj[timestamp];
                return {
                    timestamp: parseInt(timestamp, 10),
                    status: statusValue !== -1 ? parseFloat(statusValue) : null,
                };
            });

            result.push({
                adnl,
                stake: validatorMeta.stake,
                weight: validatorMeta.weight,
                index: validatorMeta.index,
                walletAddress: validatorMeta.wallet_address,
                avgEfficiency: parseFloat(validatorMeta.avg_efficiency).toFixed(2),
                statuses: statusesArray,
            });
        });

        return result;
    };


    return (
        <div id="container">
            <Header/>
            {!adnl ? (
                <Controls
                    from={from}
                    to={to}
                    cycleID={cycleID}
                    setFrom={setFrom}
                    setTo={setTo}
                    setCycleID={setCycleID}
                />
            ) : (
                ''
            )}
            {!adnl ? (
                <ValidatorList
                    from={from}
                    to={to}
                    cycleID={cycleID}
                    validators={validators}
                    sortConfig={sortConfig}
                    setSortConfig={setSortConfig}
                />
            ) : (
                <>
                    <EfficiencyChart data={efficiencyData} adnl={adnl}/>
                    <Timeline efficiencyData={efficiencyData}/>
                </>
            )}
        </div>
    );
}

export default App;
