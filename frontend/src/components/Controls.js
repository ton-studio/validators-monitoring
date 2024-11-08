// src/components/Controls.js

import React, { useState } from 'react';
import { useNavigate } from 'react-router-dom';

function Controls({ from, to, cycleID, setFrom, setTo, setCycleID }) {
    const navigate = useNavigate();
    const [activePreset, setActivePreset] = useState(null); // Track the active preset

    const presets = [
        { label: '1H', hours: 1 },
        { label: '1D', hours: 24 },
        { label: '1W', hours: 168 },
        { label: '1M', hours: 720 },
    ];

    const applyPreset = (preset) => {
        setActivePreset(preset.label); // Set the clicked preset as active

        const toDate = new Date();
        const fromDate = new Date(toDate.getTime() - preset.hours * 60 * 60 * 1000);
        const fromTimestamp = Math.floor(fromDate.getTime() / 1000).toString();
        const toTimestamp = Math.floor(toDate.getTime() / 1000).toString();
        setFrom(fromTimestamp);
        setTo(toTimestamp);

        const params = new URLSearchParams();
        params.set('from', fromTimestamp);
        params.set('to', toTimestamp);
        if (cycleID) params.set('cycle_id', cycleID);
        navigate({ search: `?${params.toString()}` });
    };

    const handleFetchData = () => {
        const params = new URLSearchParams();
        params.set('from', from);
        params.set('to', to);
        if (cycleID) params.set('cycle_id', cycleID);
        navigate({ search: `?${params.toString()}` });
    };

    return (
        <div id="controls">
            <div className="presets">
                {presets.map((preset) => (
                    <button
                        key={preset.label}
                        className={`preset-button ${activePreset === preset.label ? 'active' : ''}`}
                        onClick={() => applyPreset(preset)}
                    >
                        {preset.label}
                    </button>
                ))}
            </div>
        </div>
    );
}

export default Controls;
