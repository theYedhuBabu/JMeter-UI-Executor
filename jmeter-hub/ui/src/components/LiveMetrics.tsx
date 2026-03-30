import { useEffect, useState, useRef } from 'react'
import { MetricPayload } from '../hooks/useAgentSocket'
import { Activity, Users, CheckCircle, XCircle, Zap } from 'lucide-react'

interface LiveMetricsProps {
    metrics: MetricPayload[]
    isRunning: boolean
}

export function LiveMetrics({ metrics, isRunning }: LiveMetricsProps) {


    const [latestMetrics, setLatestMetrics] = useState({
        activeThreads: 0,
        requestsAll: 0,
        successCount: 0,
        errorCount: 0,
        throughput: 0,
    })

    const threadsPerAgent = useRef<Record<string, number>>({})

    useEffect(() => {

        if (!metrics || metrics.length === 0) return

        const latest = metrics[metrics.length - 1]?.data
        if (!latest) return

        const requests = latest.requests || 0
        const errors = latest.errors || 0
        const threads = latest.threads || 0
        const agentId = latest.agent_id || "unknown"

        // store latest thread count per agent
        threadsPerAgent.current[agentId] = threads

        // sum threads across agents
        const totalThreads = Object.values(threadsPerAgent.current)
            .reduce((sum, t) => sum + t, 0)

        setLatestMetrics(prev => {

            const newTotalRequests = prev.requestsAll + requests
            const newTotalErrors = prev.errorCount + errors
            const newSuccess = newTotalRequests - newTotalErrors

            return {
                activeThreads: totalThreads,
                requestsAll: newTotalRequests,
                successCount: newSuccess,
                errorCount: newTotalErrors,
                throughput: requests
            }
        })

    }, [metrics])

    const errorRate = latestMetrics.requestsAll > 0
        ? ((latestMetrics.errorCount / latestMetrics.requestsAll) * 100).toFixed(1)
        : '0.0'

    if (!isRunning && metrics.length === 0) return null

    return (
        <div className="bg-gray-900 rounded-xl shadow-lg border border-gray-700 overflow-hidden text-gray-200">

            <div className="bg-gray-800 px-4 py-3 border-b border-gray-700 flex items-center justify-between">

                <div className="flex items-center gap-2">
                    <Activity className="w-5 h-5 text-blue-400" />
                    <h3 className="font-semibold font-mono text-sm tracking-wide text-gray-100">
                        LIVE METRICS STREAM
                    </h3>
                </div>

                {isRunning && (
                    <div className="flex items-center gap-2">
                        <span className="relative flex h-2.5 w-2.5">
                            <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-blue-400 opacity-75"></span>
                            <span className="relative inline-flex rounded-full h-2.5 w-2.5 bg-blue-500"></span>
                        </span>
                        <span className="text-xs text-gray-400 font-mono">
                            Receiving Metrics...
                        </span>
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
                    highlight={parseFloat(errorRate) > 5}
                />

            </div>
        </div>
    )
}

function MetricCard({
    icon,
    title,
    value,
    highlight = false
}: {
    icon: React.ReactNode
    title: string
    value: string
    highlight?: boolean
}) {

    return (
        <div className="p-4 flex flex-col justify-center gap-2">

            <div className="flex items-center gap-2 text-gray-400">
                {icon}
                <span className="text-xs uppercase font-semibold tracking-wider font-mono">
                    {title}
                </span>
            </div>

            <div className={`text-2xl font-bold font-mono ${highlight ? 'text-red-400' : 'text-white'}`}>
                {value}
            </div>

        </div>
    )
}