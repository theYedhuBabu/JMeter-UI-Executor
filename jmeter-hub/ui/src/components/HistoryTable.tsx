import { useEffect, useState } from 'react';
import { getHistory, TestRun } from '../services/api';
import { FileText, ExternalLink } from 'lucide-react';

export function HistoryTable() {
    const [history, setHistory] = useState<TestRun[]>([]);
    const [loading, setLoading] = useState(true);
    const [error, setError] = useState<string | null>(null);

    useEffect(() => {
        const fetchHistory = async () => {
            try {
                const data = await getHistory();
                setHistory(data || []);
                setError(null);
            } catch (err: any) {
                setError(err.message || 'Failed to load history');
            } finally {
                setLoading(false);
            }
        };

        fetchHistory();
    }, []);

    const formatDate = (dateString?: string) => {
        if (!dateString || dateString === '0001-01-01T00:00:00Z') return '-';
        // Format into standard localized format
        return new Date(dateString).toLocaleString();
    };

    const getStatusColor = (status: string) => {
        // Basic status coloring mapped to standardized potential statuses
        switch (status?.toLowerCase()) {
            case 'completed': return 'bg-green-100 text-green-800 border-green-200';
            case 'running': return 'bg-blue-100 text-blue-800 border-blue-200';
            case 'failed': return 'bg-red-100 text-red-800 border-red-200';
            case 'stopped': return 'bg-yellow-100 text-yellow-800 border-yellow-200';
            default: return 'bg-gray-100 text-gray-800 border-gray-200';
        }
    };

    if (loading) {
        return (
            <div className="flex justify-center p-12">
                <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-blue-600"></div>
            </div>
        );
    }

    if (error) {
        return (
            <div className="bg-red-50 border border-red-200 text-red-700 px-4 py-3 rounded relative" role="alert">
                <strong className="font-bold">Error loading history: </strong>
                <span className="block sm:inline">{error}</span>
            </div>
        );
    }

    return (
        <div className="bg-white rounded-xl shadow-sm border border-gray-200 overflow-hidden">
            <div className="p-6 border-b border-gray-200 bg-gray-50 flex items-center gap-2">
                <FileText className="w-5 h-5 text-gray-500" />
                <h2 className="text-lg font-semibold text-gray-800">Test Execution History</h2>
            </div>
            <div className="overflow-x-auto">
                <table className="w-full text-left text-sm text-gray-600">
                    <thead className="bg-gray-50 text-gray-700 font-medium border-b border-gray-200">
                        <tr>
                            <th className="px-6 py-4">Run ID</th>
                            <th className="px-6 py-4">Script Name</th>
                            <th className="px-6 py-4">Status</th>
                            <th className="px-6 py-4">Start Time</th>
                            <th className="px-6 py-4">End Time</th>
                            <th className="px-6 py-4 text-right">Actions</th>
                        </tr>
                    </thead>
                    <tbody className="divide-y divide-gray-100">
                        {history.length === 0 ? (
                            <tr>
                                <td colSpan={6} className="px-6 py-12 text-center text-gray-500">
                                    <div className="flex flex-col items-center gap-2">
                                        <FileText className="w-8 h-8 text-gray-400" />
                                        <p>No previous test runs found.</p>
                                    </div>
                                </td>
                            </tr>
                        ) : (
                            history.map((run) => (
                                <tr key={run.ID} className="hover:bg-gray-50 transition-colors">
                                    <td className="px-6 py-4 font-medium text-gray-900">#{run.ID}</td>
                                    <td className="px-6 py-4 font-mono text-xs">{run.ScriptName || '-'}</td>
                                    <td className="px-6 py-4">
                                        <span className={`inline-flex items-center px-2.5 py-1 rounded-full text-xs font-medium border ${getStatusColor(run.Status)}`}>
                                            {run.Status || 'Unknown'}
                                        </span>
                                    </td>
                                    <td className="px-6 py-4">{formatDate(run.StartTime)}</td>
                                    <td className="px-6 py-4">{formatDate(run.EndTime)}</td>
                                    <td className="px-6 py-4 text-right">
                                        <button
                                            className="inline-flex items-center justify-center gap-1.5 px-3 py-1.5 text-sm font-medium text-blue-600 bg-blue-50 border border-blue-200 rounded-lg hover:bg-blue-100 transition-colors disabled:opacity-50 disabled:cursor-not-allowed disabled:bg-gray-50 disabled:text-gray-400 disabled:border-gray-200"
                                            disabled={run.Status?.toLowerCase() !== 'completed'}
                                            title={run.Status?.toLowerCase() !== 'completed' ? 'Report not available yet' : 'View Report'}
                                            onClick={() => {
                                                window.open(`http://localhost:8080/reports/${run.ID}`, '_blank');
                                            }}
                                        >
                                            <ExternalLink className="w-4 h-4" />
                                            View Report
                                        </button>
                                    </td>
                                </tr>
                            ))
                        )}
                    </tbody>
                </table>
            </div>
        </div>
    );
}
