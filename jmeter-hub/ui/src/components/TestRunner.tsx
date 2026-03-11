import { useState, useEffect, useRef } from 'react';
import { Upload, Play, Square, Server, FileText, FileSpreadsheet } from 'lucide-react';
import { getAgents, uploadScript, parseJmx, getActiveRun } from '../services/api';
import { useAgentSocket } from '../hooks/useAgentSocket';
import { LiveTerminal } from './LiveTerminal';
import { LiveMetrics } from './LiveMetrics';
import { Settings2 } from 'lucide-react';

export function TestRunnerInner({ initialRunId }: { initialRunId?: string }) {
    const [agents, setAgents] = useState<string[]>([]);
    const [selectedAgent, setSelectedAgent] = useState<string>('');
    const [selectedAgents, setSelectedAgents] = useState<string[]>([]);
    const [scriptFile, setScriptFile] = useState<File | null>(null);

    // Dynamic CSV States
    const [requiredCSVs, setRequiredCSVs] = useState<string[]>([]);
    const [mappedCsvFiles, setMappedCsvFiles] = useState<Record<string, File>>({});
    const [csvStrategies, setCsvStrategies] = useState<Record<string, 'share' | 'split'>>({});
    const [hasCsvHeader, setHasCsvHeader] = useState<boolean>(true);

    const [isRunning, setIsRunning] = useState(!!initialRunId);
    const [isUploading, setIsUploading] = useState(false);
    const [isParsing, setIsParsing] = useState(false);
    const [activeRunId, setActiveRunId] = useState<string>(initialRunId || '');
    const [executionMode, setExecutionMode] = useState<'single' | 'distributed'>('single');

    // Determine WebSocket URL dynamically based on environment
    const wsProtocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const wsHost = import.meta.env.DEV ? 'localhost:8080' : window.location.host;
    const wsUrl = `${wsProtocol}//${wsHost}/ws?agentId=web-ui`;

    // Use the custom socket hook
    const { connectionStatus, logs, metrics, runStatus, sendCommand } = useAgentSocket(wsUrl, initialRunId);

    useEffect(() => {
        if (runStatus === 'completed' || runStatus === 'failed' || runStatus === 'stopped') {
            setIsRunning(false);
        }
    }, [runStatus]);

    const scriptInputRef = useRef<HTMLInputElement>(null);

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

    const toggleAgent = (agent: string) => {
        setSelectedAgents(prev =>
            prev.includes(agent)
                ? prev.filter(a => a !== agent)
                : [...prev, agent]
        );
    };

    const handleScriptSelect = async (e: React.ChangeEvent<HTMLInputElement>) => {
        const file = e.target.files?.[0];
        if (!file) return;

        setScriptFile(file);

        // Reset CSV states
        setRequiredCSVs([]);
        setMappedCsvFiles({});
        setCsvStrategies({});

        try {
            setIsParsing(true);
            const csvVars = await parseJmx(file);
            setRequiredCSVs(csvVars);

            // Initialize default strategies and file mapping
            const initStrategies: Record<string, 'share' | 'split'> = {};
            csvVars.forEach(v => {
                initStrategies[v] = 'share';
            });
            setCsvStrategies(initStrategies);
        } catch (err: any) {
            alert("Failed to parse JMX properties: " + err.message);
        } finally {
            setIsParsing(false);
        }
    };

    const handleExecute = async (agentsToRun: string[]) => {
        if (!scriptFile) {
            alert("Please select a .jmx script file first.");
            return;
        }

        if (!agentsToRun || agentsToRun.length === 0) {
            alert("No agent selected.");
            return;
        }

        // Validate that all required CSVs have been mapped
        for (const csvVar of requiredCSVs) {
            if (!mappedCsvFiles[csvVar]) {
                alert(`Missing required CSV file for variable: ${csvVar}`);
                return;
            }
        }

        try {
            setIsUploading(true);

            // Construct Config string
            const config = JSON.stringify({
                mode: executionMode,
                agents: agentsToRun,
                csvStrategies: csvStrategies,
                hasCsvHeader: hasCsvHeader
            });

            const data = await uploadScript(scriptFile, mappedCsvFiles, config);

            setIsUploading(false);
            setActiveRunId(data.runId);

            // The backend UploadScriptHandler now inherently sends the WebSocket START command
            // so we don't need to manually iterate and sendCommand here anymore.

            setIsRunning(true);

        } catch (err: any) {
            setIsUploading(false);
            alert("Failed to start test execution: " + err.message);
        }
    };

    const handleStop = () => {
        // Send stop command to ALL globally connected agents to ensure
        // robust behavior across page reloads and mixed execution states.
        agents.forEach(agent => {
            sendCommand({
                action: 'stop',
                targetAgentId: agent,
                run_id: activeRunId
            });
        });

        setIsRunning(false);
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
                                onChange={handleScriptSelect}
                            />
                        </div>

                        {isParsing && (
                            <div className="flex items-center gap-2 text-blue-600 text-sm font-medium p-2">
                                <div className="animate-spin rounded-full h-4 w-4 border-b-2 border-blue-600"></div>
                                Parsing JMX for file requirements...
                            </div>
                        )}

                        {requiredCSVs.length > 0 && (
                            <div className="border border-purple-200 bg-purple-50/30 rounded-lg p-5 flex flex-col gap-4">
                                <h4 className="font-semibold text-purple-900 flex items-center gap-2">
                                    <FileSpreadsheet className="w-5 h-5" /> Required CSV Data Sets
                                </h4>
                                <div className="flex flex-col gap-3">
                                    {requiredCSVs.map(csvVar => (
                                        <div key={csvVar} className="bg-white border text-sm border-gray-200 rounded p-4 flex flex-col sm:flex-row sm:items-center justify-between gap-4 shadow-sm hover:border-purple-300 transition-colors">
                                            <div className="flex flex-col gap-2 flex-grow">
                                                <div className="flex items-center gap-2">
                                                    <span className="font-mono text-xs bg-gray-100 text-gray-700 px-2 py-1 rounded border border-gray-200">
                                                        {csvVar}
                                                    </span>
                                                </div>
                                                <input
                                                    type="file"
                                                    accept=".csv"
                                                    onChange={(e) => {
                                                        const file = e.target.files?.[0];
                                                        if (file) {
                                                            setMappedCsvFiles(prev => ({ ...prev, [csvVar]: file }));
                                                        }
                                                    }}
                                                    className="w-full text-sm text-gray-500 file:mr-4 file:py-2 file:px-4 file:rounded-full file:border-0 file:text-sm file:font-semibold file:bg-purple-50 file:text-purple-700 hover:file:bg-purple-100"
                                                />
                                            </div>

                                            {/* Configuration Panel for Distributed Run */}
                                            {executionMode === 'distributed' && (
                                                <div className="flex items-center border border-gray-200 rounded bg-gray-50 p-1 flex-shrink-0">
                                                    <button
                                                        onClick={() => setCsvStrategies(prev => ({ ...prev, [csvVar]: 'share' }))}
                                                        className={`px-3 py-1.5 text-xs font-medium rounded transition-colors ${csvStrategies[csvVar] === 'share'
                                                            ? 'bg-white shadow text-gray-900 border border-gray-200'
                                                            : 'text-gray-500 hover:text-gray-700'
                                                            }`}
                                                        title="Distribute full exact file copy to all agents"
                                                    >
                                                        Share full
                                                    </button>
                                                    <button
                                                        onClick={() => setCsvStrategies(prev => ({ ...prev, [csvVar]: 'split' }))}
                                                        className={`px-3 py-1.5 text-xs font-medium rounded transition-colors flex items-center gap-1 ${csvStrategies[csvVar] === 'split'
                                                            ? 'bg-purple-600 text-white shadow border border-purple-700'
                                                            : 'text-gray-500 hover:text-gray-700'
                                                            }`}
                                                        title="Split CSV logically across agents"
                                                    >
                                                        <Settings2 className="w-3 h-3" /> Split chunks
                                                    </button>
                                                </div>
                                            )}
                                        </div>
                                    ))}
                                    {/* Global CSV Options */}
                                    {executionMode === 'distributed' && Object.values(csvStrategies).includes('split') && (
                                        <div className="bg-white border text-sm border-gray-200 rounded p-4 flex items-center justify-between shadow-sm">
                                            <div className="flex flex-col gap-1">
                                                <span className="font-semibold text-gray-700">CSV Files Configuration</span>
                                                <span className="text-xs text-gray-500">How to handle headers when splitting files across agents</span>
                                            </div>
                                            <label className="flex items-center gap-2 cursor-pointer group">
                                                <input
                                                    type="checkbox"
                                                    checked={hasCsvHeader}
                                                    onChange={(e) => setHasCsvHeader(e.target.checked)}
                                                    className="w-4 h-4 text-purple-600 rounded border-gray-300 focus:ring-purple-500"
                                                />
                                                <span className="text-sm font-medium text-gray-700 group-hover:text-purple-700 transition-colors">Has Header Row</span>
                                            </label>
                                        </div>
                                    )}
                                </div>
                            </div>
                        )}
                    </div>
                </div>

                {/* Execution Control Panel */}
                <div className="bg-white rounded-xl shadow-sm border border-gray-200 flex flex-col">
                    {/* Tab Header */}
                    <div className="flex border-b border-gray-200">
                        <button
                            onClick={() => setExecutionMode('single')}
                            className={`flex-1 py-3 text-sm font-semibold transition-all ${executionMode === 'single'
                                ? 'border-b-2 border-purple-500 text-purple-600 bg-purple-50/30'
                                : 'text-gray-500 hover:bg-gray-50'
                                }`}
                        >
                            Single Machine
                        </button>
                        <button
                            onClick={() => setExecutionMode('distributed')}
                            className={`flex-1 py-3 text-sm font-semibold transition-all ${executionMode === 'distributed'
                                ? 'border-b-2 border-purple-500 text-purple-600 bg-purple-50/30'
                                : 'text-gray-500 hover:bg-gray-50'
                                }`}
                        >
                            Distributed Test
                        </button>
                    </div>

                    {/* Tab Content */}
                    <div className="p-6 flex flex-col gap-6 flex-1 min-h-[250px]">
                        {executionMode === 'single' ? (
                            <>
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

                                <div className="mt-auto">
                                    {!isRunning ? (
                                        <button
                                            onClick={() => handleExecute([selectedAgent])}
                                            disabled={isUploading || connectionStatus !== 'connected'}
                                            className="w-full bg-green-600 hover:bg-green-700 text-white font-bold py-3 px-4 rounded-lg flex items-center justify-center gap-2 transition-colors disabled:opacity-50 disabled:cursor-not-allowed">
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
                                            className="w-full bg-red-600 hover:bg-red-700 text-white font-bold py-3 px-4 rounded-lg flex items-center justify-center gap-2 transition-colors">
                                            <Square className="w-5 h-5 fill-current" />
                                            Stop Test
                                        </button>
                                    )}
                                </div>
                            </>
                        ) : (
                            <div className="animate-in fade-in slide-in-from-bottom-2 duration-300">
                                <>
                                    <div>
                                        <h3 className="font-semibold text-gray-700 flex items-center gap-2 mb-3">
                                            <Server className="w-5 h-5 text-purple-500" /> Select Agents
                                        </h3>

                                        <div className="flex flex-col gap-3 max-h-[160px] overflow-y-auto pr-2 custom-scrollbar">

                                            {agents.length === 0 ? (
                                                <div className="text-sm text-gray-500 italic px-2 py-4 text-center border border-dashed border-gray-200 rounded-lg">
                                                    No agents are currently connected.
                                                </div>
                                            ) : (
                                                agents.map(agent => {
                                                    const isSelected = selectedAgents.includes(agent);
                                                    return (
                                                        <div
                                                            key={agent}
                                                            onClick={() => !isRunning && toggleAgent(agent)}
                                                            className={`
                                                                flex items-center gap-3 p-3 rounded-lg border transition-all cursor-pointer
                                                                ${isRunning ? 'opacity-50 cursor-not-allowed' : 'hover:shadow-sm'}
                                                                ${isSelected
                                                                    ? 'border-purple-500 bg-purple-50'
                                                                    : 'border-gray-200 hover:border-purple-300 bg-white'
                                                                }
                                                            `}
                                                        >
                                                            <div className={`
                                                                w-5 h-5 rounded flex items-center justify-center border transition-colors
                                                                ${isSelected
                                                                    ? 'bg-purple-600 border-purple-600 text-white'
                                                                    : 'border-gray-300 bg-white'
                                                                }
                                                            `}>
                                                                {isSelected && <svg className="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={3}><path strokeLinecap="round" strokeLinejoin="round" d="M5 13l4 4L19 7" /></svg>}
                                                            </div>
                                                            <span className={`text-sm font-medium ${isSelected ? 'text-purple-900' : 'text-gray-700'}`}>
                                                                {agent}
                                                            </span>
                                                        </div>
                                                    );
                                                })
                                            )}

                                        </div>
                                    </div>

                                    <div className="mt-auto">
                                        {!isRunning ? (
                                            <button
                                                onClick={() => handleExecute(selectedAgents)}
                                                disabled={
                                                    isUploading ||
                                                    connectionStatus !== 'connected' ||
                                                    selectedAgents.length === 0
                                                }
                                                className="w-full bg-green-600 hover:bg-green-700 text-white font-bold py-3 px-4 rounded-lg flex items-center justify-center gap-2 transition-colors disabled:opacity-50"
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
                                                className="w-full bg-red-600 hover:bg-red-700 text-white font-bold py-3 px-4 rounded-lg flex items-center justify-center gap-2"
                                            >
                                                <Square className="w-5 h-5 fill-current" />
                                                Stop Test
                                            </button>
                                        )}
                                    </div>
                                </>

                            </div>
                        )}
                    </div>
                </div>
            </div>

            {/* Real-time Streaming Output */}
            <div className="flex flex-col gap-4">
                <LiveMetrics metrics={metrics} isRunning={isRunning} />
                <LiveTerminal logs={logs} isRunning={isRunning} />
            </div>
        </div>
    );
}

export function TestRunner() {
    const [isInitializing, setIsInitializing] = useState(true);
    const [recoveredRunId, setRecoveredRunId] = useState<string | undefined>();

    useEffect(() => {
        const checkActiveRun = async () => {
            try {
                const data = await getActiveRun();
                if (data.active && data.runId) {
                    setRecoveredRunId(data.runId);
                }
            } catch (err) {
                console.error("Failed to fetch active run status", err);
            } finally {
                setIsInitializing(false);
            }
        };

        checkActiveRun();
    }, []);

    if (isInitializing) {
        return (
            <div className="flex h-full items-center justify-center bg-gray-50 rounded-xl border border-gray-200">
                <div className="flex flex-col items-center gap-4">
                    <div className="animate-spin rounded-full h-10 w-10 border-b-2 border-purple-600"></div>
                    <span className="text-gray-600 font-medium animate-pulse">Synchronizing with UI Hub...</span>
                </div>
            </div>
        );
    }

    return <TestRunnerInner initialRunId={recoveredRunId} />;
}
