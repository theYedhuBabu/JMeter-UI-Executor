import { useEffect, useState, useRef } from 'react';
import { MetricPayload } from '../hooks/useAgentSocket';
import { Activity, Users, CheckCircle, XCircle, Zap } from 'lucide-react';

interface LiveMetricsProps {
    metrics: MetricPayload[];
    isRunning: boolean;
}

export function LiveMetrics({ metrics, isRunning }: LiveMetricsProps) {
    const lastStatsRef = useRef({ time: Date.now(), requestsAll: 0 });
    const [latestMetrics, setLatestMetrics] = useState({
        activeThreads: 0,
        requestsAll: 0,
        successCount: 0,
        errorCount: 0,
        responseAvg: 0,
        throughput: 0,
    });

    useEffect(() => {
        if (!metrics || metrics.length === 0) return;

        // Reset stats to 0, then rebuild from the metrics array
        // because we sum them incrementally to get total requests.
        const newStats = {
            activeThreads: 0,
            requestsAll: 0,
            successCount: 0,
            errorCount: 0,
            responseAvg: 0,
        };

        const threadsPerAgent: Record<string, number> = {};

        for (let i = 0; i < metrics.length; i++) {
            const m = metrics[i].data;
            if (!m) continue;

            const appMatch = m.match(/application=jmeter_([^, ]+)/);
            const agentId = appMatch ? appMatch[1] : 'unknown';

            if (m.includes('transaction=internal')) {
                const match = m.match(/meanAT=([\d.]+)/);
                if (match) {
                    threadsPerAgent[agentId] = parseFloat(match[1]); // Gauge, stores latest per agent
                }
            }

            if (m.includes('transaction=all')) {
                const countMatch = m.match(/count=([\d.]+)/);
                const errorMatch = m.match(/countError=([\d.]+)/);

                const c = countMatch ? parseFloat(countMatch[1]) : 0;
                const e = errorMatch ? parseFloat(errorMatch[1]) : 0;

                newStats.requestsAll += c;
                newStats.errorCount += e;
                newStats.successCount += (c - e);
            }
        }

        let totalThreads = 0;
        Object.values(threadsPerAgent).forEach(val => {
            totalThreads += val;
        });
        newStats.activeThreads = totalThreads;

        let newThroughput = latestMetrics.throughput;
        const now = Date.now();
        const timeDiff = (now - lastStatsRef.current.time) / 1000;

        if (newStats.requestsAll < lastStatsRef.current.requestsAll || newStats.requestsAll === 0) {
            lastStatsRef.current = { time: now, requestsAll: newStats.requestsAll };
            newThroughput = 0;
        } else if (timeDiff >= 2.0) {
            const delta = newStats.requestsAll - lastStatsRef.current.requestsAll;
            newThroughput = delta / timeDiff;
            lastStatsRef.current = { time: now, requestsAll: newStats.requestsAll };
        }

        setLatestMetrics({
            ...newStats,
            throughput: newThroughput
        });

        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [metrics]);

    const errorRate = latestMetrics.requestsAll > 0
        ? ((latestMetrics.errorCount / latestMetrics.requestsAll) * 100).toFixed(1)
        : '0.0';

    if (!isRunning && metrics.length === 0) return null;

    return (
        <div className="bg-gray-900 rounded-xl shadow-lg border border-gray-700 overflow-hidden text-gray-200">
            <div className="bg-gray-800 px-4 py-3 border-b border-gray-700 flex items-center justify-between">
                <div className="flex items-center gap-2">
                    <Activity className="w-5 h-5 text-blue-400" />
                    <h3 className="font-semibold font-mono text-sm tracking-wide text-gray-100">LIVE METRICS STREAM</h3>
                </div>
                {isRunning && (
                    <div className="flex items-center gap-2">
                        <span className="relative flex h-2.5 w-2.5">
                            <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-blue-400 opacity-75"></span>
                            <span className="relative inline-flex rounded-full h-2.5 w-2.5 bg-blue-500"></span>
                        </span>
                        <span className="text-xs text-gray-400 font-mono">Receiving Influx Data...</span>
                    </div>
                )}
            </div>

            <div className="grid grid-cols-2 md:grid-cols-5 divide-y md:divide-y-0 md:divide-x divide-gray-700">
                <MetricCard
                    icon={<Users className="w-5 h-5 text-indigo-400" />}
                    title="Active Threads"
                    value={latestMetrics.activeThreads.toString()}
                />
                <MetricCard
                    icon={<Zap className="w-5 h-5 text-yellow-400" />}
                    title="Throughput"
                    value={`${latestMetrics.throughput.toFixed(1)}/s`}
                />
                <MetricCard
                    icon={<Activity className="w-5 h-5 text-blue-400" />}
                    title="Total Requests"
                    value={latestMetrics.requestsAll.toString()}
                />
                <MetricCard
                    icon={<CheckCircle className="w-5 h-5 text-green-400" />}
                    title="Success Rate"
                    value={`${(100 - parseFloat(errorRate)).toFixed(1)}%`}
                />
                <MetricCard
                    icon={<XCircle className="w-5 h-5 text-red-400" />}
                    title="Error Rate"
                    value={`${errorRate}%`}
                    highlight={parseFloat(errorRate) > 5.0} // turn red if > 5% errors
                />
            </div>
        </div>
    );
}

function MetricCard({ icon, title, value, highlight = false }: { icon: React.ReactNode, title: string, value: string, highlight?: boolean }) {
    return (
        <div className="p-4 flex flex-col justify-center gap-2">
            <div className="flex items-center gap-2 text-gray-400">
                {icon}
                <span className="text-xs uppercase font-semibold tracking-wider font-mono">{title}</span>
            </div>
            <div className={`text-2xl font-bold font-mono ${highlight ? 'text-red-400' : 'text-white'}`}>
                {value}
            </div>
        </div>
    );
}
