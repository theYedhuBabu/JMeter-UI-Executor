import { useState, useEffect, useRef } from 'react';
import { Upload, Play, Square, Server, FileText, FileSpreadsheet } from 'lucide-react';
import { getAgents, uploadScript } from '../services/api';
import { useAgentSocket } from '../hooks/useAgentSocket';
import { LiveTerminal } from './LiveTerminal';

export function TestRunner() {
    const [agents, setAgents] = useState<string[]>([]);
    const [selectedAgent, setSelectedAgent] = useState<string>('');
    const [scriptFile, setScriptFile] = useState<File | null>(null);
    const [csvFiles, setCsvFiles] = useState<File[]>([]);
    const [isRunning, setIsRunning] = useState(false);
    const [isUploading, setIsUploading] = useState(false);

    // Use the custom socket hook
    const { connectionStatus, logs, sendCommand } = useAgentSocket('ws://localhost:8080/ws?agentId=web-ui');

    const scriptInputRef = useRef<HTMLInputElement>(null);
    const csvInputRef = useRef<HTMLInputElement>(null);

    // Fetch available agents on component mount
    useEffect(() => {
        const fetchAgents = async () => {
            try {
                const data = await getAgents();
                setAgents(data);
                if (data.length > 0 && !selectedAgent) {
                    setSelectedAgent(data[0]);
                }
            } catch (err) {
                console.error("Failed to fetch agents", err);
            }
        };
        fetchAgents();
        // Poll for agents every 5 seconds just in case new ones connect
        const interval = setInterval(fetchAgents, 5000);
        return () => clearInterval(interval);
    }, [selectedAgent]);

    const handleExecute = async () => {
        if (!scriptFile) {
            alert("Please select a .jmx script file first.");
            return;
        }
        if (!selectedAgent) {
            alert("No agent selected or available. Please wait for an agent to connect.");
            return;
        }

        try {
            setIsUploading(true);
            const data = await uploadScript(scriptFile, csvFiles);
            setIsUploading(false);

            // Fire the start command via WebSocket
            sendCommand({
                action: 'start',
                agentId: selectedAgent,
                run_id: data.runId,
                download_urls: {
                    jmx: data.downloadURLs[0]
                }
            });
            setIsRunning(true);
        } catch (err: any) {
            setIsUploading(false);
            alert("Failed to start test execution: " + err.message);
        }
    };

    const handleStop = () => {
        if (selectedAgent) {
            sendCommand({ action: 'stop', agentId: selectedAgent });
            setIsRunning(false);
        }
    };

    return (
        <div className="flex flex-col h-full gap-6">
            <div className="flex items-center justify-between pb-4 border-b border-gray-200">
                <h2 className="text-2xl font-bold text-gray-800">Test Execution</h2>
                <div className="flex items-center gap-2">
                    <div className={`w-3 h-3 rounded-full ${connectionStatus === 'connected' ? 'bg-green-500' : 'bg-red-500'}`}></div>
                    <span className="text-sm font-medium text-gray-600">Hub connection: {connectionStatus}</span>
                </div>
            </div>

            <div className="grid grid-cols-1 lg:grid-cols-3 gap-6 flex-shrink-0">

                {/* Upload Zone */}
                <div className="lg:col-span-2 bg-white rounded-xl shadow-sm border border-gray-200 p-6 flex flex-col gap-4">
                    <h3 className="font-semibold text-gray-700 flex items-center gap-2">
                        <Upload className="w-5 h-5 text-blue-500" /> Upload Test Assets
                    </h3>

                    <div className="flex flex-col gap-4">
                        <div
                            className="border-2 border-dashed border-gray-300 rounded-lg p-6 flex flex-col items-center justify-center hover:bg-gray-50 transition-colors cursor-pointer"
                            onClick={() => scriptInputRef.current?.click()}
                        >
                            <FileText className="w-10 h-10 text-gray-400 mb-2" />
                            <p className="text-sm font-medium text-gray-700">
                                {scriptFile ? scriptFile.name : "Click to select a .jmx script"}
                            </p>
                            <p className="text-xs text-gray-500 mt-1">Required</p>
                            <input
                                type="file"
                                ref={scriptInputRef}
                                className="hidden"
                                accept=".jmx"
                                onChange={(e) => {
                                    if (e.target.files?.[0]) setScriptFile(e.target.files[0]);
                                }}
                            />
                        </div>

                        <div
                            className="border border-gray-200 rounded-lg p-4 flex flex-col gap-2 hover:bg-gray-50 transition-colors cursor-pointer"
                            onClick={() => csvInputRef.current?.click()}
                        >
                            <div className="flex items-center gap-2 text-gray-700">
                                <FileSpreadsheet className="w-5 h-5 text-green-500" />
                                <span className="text-sm font-medium">Additional CSV Data Files</span>
                            </div>
                            <p className="text-xs text-gray-500">
                                {csvFiles.length > 0
                                    ? `${csvFiles.length} files selected (${csvFiles.map((f: File) => f.name).join(', ')})`
                                    : "Optional: Click to select CSV files used in your test plan"}
                            </p>
                            <input
                                type="file"
                                ref={csvInputRef}
                                className="hidden"
                                multiple
                                accept=".csv"
                                onChange={(e) => {
                                    if (e.target.files) setCsvFiles(Array.from(e.target.files));
                                }}
                            />
                        </div>
                    </div>
                </div>

                {/* Execution Control Panel */}
                <div className="bg-white rounded-xl shadow-sm border border-gray-200 p-6 flex flex-col gap-6">
                    <div>
                        <h3 className="font-semibold text-gray-700 flex items-center gap-2 mb-3">
                            <Server className="w-5 h-5 text-purple-500" /> Target Agent
                        </h3>
                        <select
                            value={selectedAgent}
                            onChange={(e) => setSelectedAgent(e.target.value)}
                            className="w-full border border-gray-300 rounded-md px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
                            disabled={isRunning || agents.length === 0}
                        >
                            {agents.length === 0 ? (
                                <option value="">No agents connected</option>
                            ) : (
                                agents.map((agent: string) => (
                                    <option key={agent} value={agent}>{agent}</option>
                                ))
                            )}
                        </select>
                    </div>

                    <div className="flex-1 flex flex-col justify-end">
                        {!isRunning ? (
                            <button
                                onClick={handleExecute}
                                disabled={isUploading || connectionStatus !== 'connected'}
                                className="w-full bg-green-600 hover:bg-green-700 text-white font-bold py-3 px-4 rounded-lg flex items-center justify-center gap-2 transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
                            >
                                {isUploading ? (
                                    <div className="animate-spin rounded-full h-5 w-5 border-b-2 border-white"></div>
                                ) : (
                                    <Play className="w-5 h-5" />
                                )}
                                {isUploading ? "Uploading..." : "Execute Test"}
                            </button>
                        ) : (
                            <button
                                onClick={handleStop}
                                className="w-full bg-red-600 hover:bg-red-700 text-white font-bold py-3 px-4 rounded-lg flex items-center justify-center gap-2 transition-colors"
                            >
                                <Square className="w-5 h-5 fill-current" />
                                Stop Test
                            </button>
                        )}
                    </div>
                </div>
            </div>

            {/* Real-time Logs Console */}
            <LiveTerminal logs={logs} isRunning={isRunning} />
        </div>
    );
}
