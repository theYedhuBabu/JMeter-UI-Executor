import { useEffect, useRef, useState } from 'react';

import { LogEntry } from '../hooks/useAgentSocket';

interface LiveTerminalProps {
    logs: LogEntry[];
    isRunning: boolean;
}

export function LiveTerminal({ logs, isRunning }: LiveTerminalProps) {
    const containerRef = useRef<HTMLDivElement>(null);
    const [autoScroll, setAutoScroll] = useState(true);
    const [activeTab, setActiveTab] = useState<string>('ALL');

    const uniqueAgents = Array.from(new Set(logs.map(l => l.agentId))).filter(a => a !== 'SYSTEM' && a !== 'UNKNOWN');
    const filteredLogs = logs.filter(l => activeTab === 'ALL' || l.agentId === activeTab || l.agentId === 'SYSTEM');

    // Scroll to bottom whenever logs update, IF autoScroll is enabled
    useEffect(() => {
        if (autoScroll && containerRef.current) {
            containerRef.current.scrollTop = containerRef.current.scrollHeight;
        }
    }, [logs, autoScroll]);

    // Determine if the user has manually scrolled up
    const handleScroll = () => {
        if (!containerRef.current) return;

        const { scrollTop, scrollHeight, clientHeight } = containerRef.current;

        // If we're within 20 pixels of the bottom, consider it "at the bottom" so we auto-scroll again
        const isAtBottom = Math.abs(scrollHeight - clientHeight - scrollTop) < 20;

        if (isAtBottom && !autoScroll) {
            setAutoScroll(true);
        } else if (!isAtBottom && autoScroll) {
            setAutoScroll(false);
        }
    };

    // Basic method to style log lines
    const renderLogLine = (log: LogEntry, index: number) => {
        const text = log.text;
        const isError = text.includes('ERROR') || text.includes('Exception') || text.includes('FATAL');
        const isWarning = text.includes('WARN');

        let colorClass = 'text-gray-300';
        if (isError) colorClass = 'text-red-400 font-medium';
        else if (isWarning) colorClass = 'text-yellow-300';

        const prefix = (activeTab === 'ALL' && log.agentId && log.agentId !== 'SYSTEM' && log.agentId !== 'UNKNOWN')
            ? `[${log.agentId}] `
            : '';

        return (
            <div key={index} className={`break-words py-0.5 ${colorClass}`}>
                {prefix}{text}
            </div>
        );
    };

    return (
        <div className="flex-1 bg-gray-900 rounded-xl shadow-inner border border-gray-800 flex flex-col overflow-hidden min-h-[300px] relative">
            <div className="bg-gray-800 flex flex-col pt-2 border-b border-gray-700">
                <div className="px-4 pb-2 flex items-center justify-between">
                    <span className="text-xs font-mono text-gray-300">Live Agent Console</span>
                    <div className="flex items-center gap-4">
                        {!autoScroll && (
                            <button
                                onClick={() => setAutoScroll(true)}
                                className="text-xs text-blue-400 hover:text-blue-300 transition-colors bg-blue-500/10 px-2 py-1 rounded"
                            >
                                Resume Auto-scroll
                            </button>
                        )}
                        <div className="flex items-center gap-1.5">
                            <div className={`w-2 h-2 rounded-full ${isRunning ? 'bg-green-500 animate-pulse' : 'bg-gray-500'}`}></div>
                            <span className="text-xs text-gray-400">{isRunning ? 'Running' : 'Idle'}</span>
                        </div>
                    </div>
                </div>
                {uniqueAgents.length > 0 && (
                    <div className="flex gap-1 px-4 overflow-x-auto scrollbar-thin">
                        <button
                            onClick={() => setActiveTab('ALL')}
                            className={`px-4 py-1.5 rounded-t-md text-xs font-mono transition-colors whitespace-nowrap ${activeTab === 'ALL' ? 'bg-gray-700 text-blue-400 border-t-2 border-blue-500' : 'bg-gray-800 text-gray-500 hover:bg-gray-700 hover:text-gray-300 border-t-2 border-transparent'
                                }`}
                        >
                            ALL AGENTS
                        </button>
                        {uniqueAgents.map(agent => (
                            <button
                                key={agent}
                                onClick={() => setActiveTab(agent)}
                                className={`px-4 py-1.5 rounded-t-md text-xs font-mono transition-colors whitespace-nowrap ${activeTab === agent ? 'bg-gray-700 text-blue-400 border-t-2 border-blue-500' : 'bg-gray-800 text-gray-500 hover:bg-gray-700 hover:text-gray-300 border-t-2 border-transparent'
                                    }`}
                            >
                                {agent}
                            </button>
                        ))}
                    </div>
                )}
            </div>

            <div
                ref={containerRef}
                onScroll={handleScroll}
                className="flex-1 p-4 font-mono text-xs overflow-auto overflow-x-hidden"
            >
                {filteredLogs.length === 0 ? (
                    <div className="text-gray-600 italic">Waiting for execution logs...</div>
                ) : (
                    filteredLogs.map(renderLogLine)
                )}
            </div>
        </div>
    );
}
