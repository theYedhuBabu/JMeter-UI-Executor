import { useEffect, useState } from 'react';
import { Link } from 'react-router-dom';
import { getActiveRun, getAgents, getHistory, TestRun } from '../services/api';
import { Activity, Play, ChevronRight, Server, FileText } from 'lucide-react';

export function DashboardPage() {
    const [activeRun, setActiveRun] = useState<{ active: boolean; runId: string | null }>({ active: false, runId: null });
    const [agents, setAgents] = useState<string[]>([]);
    const [recentRuns, setRecentRuns] = useState<TestRun[]>([]);
    const [loading, setLoading] = useState(true);

    useEffect(() => {
        const fetchDashboardData = async () => {
            try {
                const [activeRunData, agentsData, historyData] = await Promise.all([
                    getActiveRun().catch(() => ({ active: false, runId: null })),
                    getAgents().catch(() => []),
                    getHistory().catch(() => [])
                ]);

                setActiveRun(activeRunData);
                setAgents(agentsData || []);
                setRecentRuns((historyData || []).slice(0, 5)); // Just top 5
            } catch (error) {
                console.error("Error fetching dashboard data:", error);
            } finally {
                setLoading(false);
            }
        };

        fetchDashboardData();
        // Poll every 5 seconds for updates
        const interval = setInterval(fetchDashboardData, 5000);
        return () => clearInterval(interval);
    }, []);

    const formatDate = (dateString?: string) => {
        if (!dateString || dateString === '0001-01-01T00:00:00Z') return '-';
        return new Date(dateString).toLocaleString(undefined, {
            month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit'
        });
    };

    const getStatusColor = (status: string) => {
        switch (status?.toLowerCase()) {
            case 'completed': return 'bg-green-100 text-green-800';
            case 'running': return 'bg-blue-100 text-blue-800';
            case 'failed': return 'bg-red-100 text-red-800';
            case 'stopped': return 'bg-yellow-100 text-yellow-800';
            default: return 'bg-gray-100 text-gray-800';
        }
    };

    if (loading) {
        return (
            <div className="flex justify-center items-center h-full">
                <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-blue-600"></div>
            </div>
        );
    }

    return (
        <div className="grid grid-cols-1 lg:grid-cols-[2fr_1fr] grid-rows-none lg:grid-rows-2 gap-6 h-full">

            {/* Active Runs */}
            <div className="bg-white rounded-xl shadow-sm border border-gray-100 p-6 flex flex-col">
                <div className="flex items-center gap-2 mb-4">
                    <Activity className="w-5 h-5 text-blue-600" />
                    <h2 className="text-lg font-semibold text-gray-800">Active Test Run</h2>
                </div>

                <div className="flex-1 flex items-center justify-center bg-gray-50 rounded-lg p-6 border border-gray-100">
                    {activeRun.active ? (
                        <div className="text-center w-full">
                            <div className="inline-flex items-center justify-center w-16 h-16 rounded-full bg-blue-100 text-blue-600 mb-4">
                                <Activity className="w-8 h-8 animate-pulse" />
                            </div>
                            <h3 className="text-xl font-bold text-gray-900 mb-2">Run #{activeRun.runId}</h3>
                            <p className="text-gray-500 mb-6">A test is currently executing</p>
                            <Link
                                to="/execution"
                                className="inline-flex items-center justify-center gap-2 px-6 py-3 text-sm font-medium text-white bg-blue-600 border border-transparent rounded-lg hover:bg-blue-700 transition"
                            >
                                <Play className="w-4 h-4" />
                                Go to Execution Dashboard
                            </Link>
                        </div>
                    ) : (
                        <div className="text-center">
                            <div className="inline-flex items-center justify-center w-12 h-12 rounded-full bg-gray-100 text-gray-400 mb-3">
                                <Activity className="w-6 h-6" />
                            </div>
                            <p className="text-gray-500">No active test runs</p>
                        </div>
                    )}
                </div>
            </div>

            {/* Active Agents */}
            <div className="bg-white rounded-xl shadow-sm border border-gray-100 p-6 lg:row-span-2 flex flex-col">
                <div className="flex items-center gap-2 mb-4">
                    <Server className="w-5 h-5 text-green-600" />
                    <h2 className="text-lg font-semibold text-gray-800">Connected Agents</h2>
                    <span className="ml-auto bg-green-100 text-green-800 text-xs font-semibold px-2.5 py-0.5 rounded-full">
                        {agents.length} Online
                    </span>
                </div>

                <div className="flex-1 overflow-y-auto">
                    {agents.length === 0 ? (
                        <div className="h-full flex flex-col items-center justify-center text-gray-500 text-sm p-4 text-center border-2 border-dashed border-gray-100 rounded-lg">
                            <Server className="w-8 h-8 text-gray-300 mb-2" />
                            <p>No agents connected to the hub.</p>
                        </div>
                    ) : (
                        <ul className="space-y-3">
                            {agents.map((agent, index) => (
                                <li key={index} className="flex items-center p-3 bg-gray-50 rounded-lg border border-gray-100">
                                    <div className="w-2 h-2 rounded-full bg-green-500 animate-pulse mr-3"></div>
                                    <span className="font-medium text-sm text-gray-700 truncate">{agent}</span>
                                </li>
                            ))}
                        </ul>
                    )}
                </div>
            </div>

            {/* Recent Runs */}
            <div className="bg-white rounded-xl shadow-sm border border-gray-100 p-6 flex flex-col">
                <div className="flex flex-row items-center justify-between mb-4">
                    <div className="flex items-center gap-2">
                        <FileText className="w-5 h-5 text-purple-600" />
                        <h2 className="text-lg font-semibold text-gray-800">Recent Runs</h2>
                    </div>
                    <Link to="/history" className="p-1 text-gray-400 hover:text-blue-600 hover:bg-blue-50 rounded transition-colors" title="View All History">
                        <ChevronRight className="w-5 h-5" />
                    </Link>
                </div>

                <div className="flex-1 overflow-x-auto">
                    {recentRuns.length === 0 ? (
                        <div className="h-full flex flex-col items-center justify-center text-gray-500 text-sm p-4 border-2 border-dashed border-gray-100 rounded-lg">
                            <p>No test history available.</p>
                        </div>
                    ) : (
                        <ul className="divide-y divide-gray-100">
                            {recentRuns.map(run => (
                                <li key={run.ID} className="py-3 flex items-center justify-between group">
                                    <div className="flex items-center gap-3">
                                        <div className={`w - 2 h - 2 rounded - full ${run.Status?.toLowerCase() === 'failed' ? 'bg-red-500' : run.Status?.toLowerCase() === 'completed' ? 'bg-green-500' : 'bg-gray-400'} `}></div>
                                        <div>
                                            <p className="text-sm font-medium text-gray-900 group-hover:text-blue-600 transition-colors">Run #{run.ID}</p>
                                            <p className="text-xs text-gray-500">{formatDate(run.StartTime)}</p>
                                        </div>
                                    </div>
                                    <span className={`inline - flex items - center px - 2 py - 0.5 rounded text - xs font - medium ${getStatusColor(run.Status)} `}>
                                        {run.Status || 'Unknown'}
                                    </span>
                                </li>
                            ))}
                        </ul>
                    )}
                </div>
            </div>

        </div>
    );
}