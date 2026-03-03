import { useEffect, useRef, useState } from 'react';

interface LiveTerminalProps {
    logs: string[];
    isRunning: boolean;
}

export function LiveTerminal({ logs, isRunning }: LiveTerminalProps) {
    const containerRef = useRef<HTMLDivElement>(null);
    const [autoScroll, setAutoScroll] = useState(true);

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
    const renderLogLine = (log: string, index: number) => {
        const isError = log.includes('ERROR') || log.includes('Exception') || log.includes('FATAL');
        const isWarning = log.includes('WARN');

        let colorClass = 'text-gray-300';
        if (isError) colorClass = 'text-red-400 font-medium';
        else if (isWarning) colorClass = 'text-yellow-300';

        return (
            <div key={index} className={`break-words py-0.5 ${colorClass}`}>
                {log}
            </div>
        );
    };

    return (
        <div className="flex-1 bg-gray-900 rounded-xl shadow-inner border border-gray-800 flex flex-col overflow-hidden min-h-[300px] relative">
            <div className="bg-gray-800 px-4 py-2 border-b border-gray-700 flex items-center justify-between">
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

            <div
                ref={containerRef}
                onScroll={handleScroll}
                className="flex-1 p-4 font-mono text-xs overflow-auto overflow-x-hidden"
            >
                {logs.length === 0 ? (
                    <div className="text-gray-600 italic">Waiting for execution logs...</div>
                ) : (
                    logs.map(renderLogLine)
                )}
            </div>
        </div>
    );
}
