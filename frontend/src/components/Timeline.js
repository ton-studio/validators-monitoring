import React from 'react';

function Timeline({ efficiencyData }) {
    if (!efficiencyData || efficiencyData.length === 0) {
        return (
            <div id="timeline">
                <div className="timeline-header">
                    <h2>Efficiency Timeline</h2>
                    <a
                        href="https://t.me/TonValidatorHealthBot?start"
                        className="subscribe-button"
                        id="subscribeButton"
                    >
                        Subscribe to alerts
                    </a>
                </div>
                <div id="timelineContent">
                    <p>No data available.</p>
                </div>
            </div>
        );
    }

    const generateTimeline = () => {
        const timelineItems = [];
        let previousStatus =
            efficiencyData[0].value >= 80 ? 'ok' : 'fail';
        let previousTimestamp = efficiencyData[0].timestamp;
        const nowTimestamp = Math.floor(Date.now() / 1000);

        efficiencyData.forEach((entry, index) => {
            const currentStatus = entry.value >= 90 ? 'ok' : 'fail';

            if (
                currentStatus !== previousStatus ||
                index === efficiencyData.length - 1
            ) {
                const currentTimestamp = entry.timestamp;
                const duration =
                    index === efficiencyData.length - 1
                        ? Math.floor((nowTimestamp - previousTimestamp) / 60)
                        : Math.floor((currentTimestamp - previousTimestamp) / 60);

                timelineItems.unshift({
                    status: previousStatus,
                    timestamp: previousTimestamp,
                    duration,
                });

                previousStatus = currentStatus;
                previousTimestamp = currentTimestamp;
            }
        });

        return timelineItems;
    };

    const timelineItems = generateTimeline();

    return (
        <div id="timeline">
            <div className="timeline-header">
                <h2>Efficiency Timeline</h2>
                <a
                    href="https://t.me/TonValidatorHealthBot?start"
                    className="subscribe-button"
                    id="subscribeButton"
                >
                    Subscribe to alerts
                </a>
            </div>
            <div id="timelineContent">
                {timelineItems.map((item, index) => (
                    <div className="timeline-item" key={index}>
                        <div
                            className={`timeline-icon ${
                                item.status === 'ok' ? 'ok' : 'fail'
                            }`}
                        ></div>
                        <div className="timeline-content">
                            {new Date(item.timestamp * 1000).toLocaleString()} -{' '}
                            <strong>{item.status.toUpperCase()}</strong>
                            <div className="timeline-duration">
                                Duration: {item.duration} minutes
                            </div>
                        </div>
                    </div>
                ))}
            </div>
        </div>
    );
}

export default Timeline;
