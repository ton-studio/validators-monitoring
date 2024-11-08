import React, { useEffect, useRef } from 'react';
import {
    Chart,
    LineElement,
    PointElement,
    LineController,
    CategoryScale,
    LinearScale,
    TimeScale,
    Tooltip,
} from 'chart.js';
import { Line } from 'react-chartjs-2';
import annotationPlugin from 'chartjs-plugin-annotation';
import 'chartjs-adapter-date-fns';

Chart.register(
    LineElement,
    PointElement,
    LineController,
    CategoryScale,
    LinearScale,
    TimeScale,
    annotationPlugin,
    Tooltip
);

function EfficiencyChart({ data, adnl }) {
    const chartRef = useRef(null);

    useEffect(() => {
        if (data && chartRef.current) {
            const chartInstance = chartRef.current;

            const labels = data.map((e) => new Date(e.timestamp * 1000));
            const values = data.map((e) => e.value);

            const annotations = {};
            let previousStatus = null;
            let previousCycleID = null;
            let annotationIndex = 0;

            data.forEach((entry, index) => {
                const currentStatus = entry.value >= 80 ? 'ok' : 'fail';
                if (currentStatus !== previousStatus) {
                    const statusTime = new Date(entry.timestamp * 1000);
                    annotations[`statusLine${annotationIndex}`] = {
                        type: 'line',
                        xMin: statusTime,
                        xMax: statusTime,
                        borderColor:
                            currentStatus === 'ok'
                                ? 'rgba(0, 255, 0, 0.7)'
                                : 'rgba(255, 0, 0, 0.7)',
                        borderWidth: 1,
                        borderDash: [5, 5],
                        label: {
                            content: currentStatus === 'ok' ? 'OK' : 'FAIL',
                            display: true,
                            position: 'start',
                            backgroundColor:
                                currentStatus === 'ok'
                                    ? 'rgba(76, 175, 80, 0.7)'
                                    : 'rgba(229, 57, 53, 0.7)',
                            color: '#FFFFFF',
                            font: {
                                size: 10,
                                weight: 'bold',
                            },
                        },
                    };
                    previousStatus = currentStatus;
                    annotationIndex++;
                }

                const currentCycleID = entry.cycle_id;
                if (currentCycleID !== previousCycleID) {
                    const statusTime = new Date(entry.timestamp * 1000);
                    annotations[`cycleLine${annotationIndex}`] = {
                        type: 'line',
                        xMin: statusTime,
                        xMax: statusTime,
                        borderColor: 'rgba(255, 255, 255, 0.5)',
                        borderWidth: 1,
                        borderDash: [5, 5],
                        label: {
                            content: currentCycleID,
                            display: true,
                            position: 'end',
                            backgroundColor: 'rgba(200, 200, 200, 0.7)',
                            color: '#FFFFFF',
                            font: {
                                size: 10,
                                weight: 'bold',
                            },
                        },
                    };
                    previousCycleID = currentCycleID;
                    annotationIndex++;
                }
            });

            chartInstance.data.labels = labels;
            chartInstance.data.datasets[0].data = values;
            chartInstance.options.plugins.annotation.annotations = annotations;
            chartInstance.update();
        }
    }, [data]);

    const chartData = {
        labels: [],
        datasets: [
            {
                label: `ADNL ${adnl}`,
                data: [],
                fill: false,
                borderColor: '#768B96',
                backgroundColor: '#768B96',
                pointRadius: 2,
                pointHoverRadius: 4,
            },
        ],
    };

    const chartOptions = {
        scales: {
            x: {
                type: 'time',
                time: {
                    unit: 'hour',
                    tooltipFormat: 'PPpp',
                },
                title: {
                    display: true,
                    text: 'Time',
                    color: '#333',
                },
                grid: {
                    color: '#ccc',
                },
                ticks: {
                    color: '#333',
                },
            },
            y: {
                min: 0,
                max: 100,
                title: {
                    display: true,
                    text: 'Efficiency (%)',
                    color: '#333',
                },
                grid: {
                    color: '#ccc',
                },
                ticks: {
                    color: '#333',
                },
            },
        },
        plugins: {
            tooltip: {
                callbacks: {
                    label: function (context) {
                        const value = context.parsed.y;
                        return `Efficiency: ${value.toFixed(2)}%`;
                    },
                },
            },
            annotation: {
                annotations: {},
            },
        },
    };

    return (
        <div id="chart-container">
            <Line ref={chartRef} data={chartData} options={chartOptions} height={100} />
        </div>
    );
}

export default EfficiencyChart;
