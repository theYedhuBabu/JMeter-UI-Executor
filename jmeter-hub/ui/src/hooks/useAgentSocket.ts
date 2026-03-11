import { useState, useEffect, useRef, useCallback } from 'react';

export type ConnectionStatus = 'connecting' | 'connected' | 'disconnected';

export interface CommandPayload {
    action: 'start' | 'stop';
    targetAgentId?: string;
    [key: string]: any;
}

export interface MetricPayload {
    type: 'metric';
    log_line: string;
    data: string;
}

export interface LogEntry {
    agentId: string;
    text: string;
}

export type AgentRunStatus = 'running' | 'completed' | 'failed' | 'stopped' | null;

export function useAgentSocket(url: string = 'ws://localhost:8080/ws', recoveredRunId?: string) {
    const [connectionStatus, setConnectionStatus] = useState<ConnectionStatus>('disconnected');
    const [logs, setLogs] = useState<LogEntry[]>([]);
    const [metrics, setMetrics] = useState<MetricPayload[]>([]);

    // Use an internal ref to access the latest runStatus inside callbacks without dependency issues
    const [runStatus, _setRunStatus] = useState<AgentRunStatus>(recoveredRunId ? 'running' : null);
    const runStatusRef = useRef<AgentRunStatus>(recoveredRunId ? 'running' : null);

    const setRunStatus = useCallback((status: AgentRunStatus) => {
        runStatusRef.current = status;
        _setRunStatus(status);
    }, []);

    const wsRef = useRef<WebSocket | null>(null);
    const reconnectTimeoutRef = useRef<NodeJS.Timeout | null>(null);
    const logBufferRef = useRef<LogEntry[]>([]);
    const MAX_LOGS = 1000;

    const connect = useCallback(() => {
        // Prevent multiple connection attempts if already open or connecting
        if (wsRef.current?.readyState === WebSocket.OPEN || wsRef.current?.readyState === WebSocket.CONNECTING) {
            return;
        }

        setConnectionStatus('connecting');
        const ws = new WebSocket(url);
        wsRef.current = ws;

        ws.onopen = () => {
            setConnectionStatus('connected');
            if (reconnectTimeoutRef.current) {
                clearTimeout(reconnectTimeoutRef.current);
                reconnectTimeoutRef.current = null;
            }
            // Clear logs only if we are starting fresh and not recovering/reconnecting to an active run
            if (runStatusRef.current !== 'running') {
                setLogs([]);
                setMetrics([]);
            }
        };

        ws.onmessage = (event) => {
            let logText = event.data;
            let agentId = 'SYSTEM';
            try {
                const parsed = JSON.parse(event.data);
                if (parsed && typeof parsed.status === 'string') {
                    setRunStatus(parsed.status as AgentRunStatus);
                    logText = `Agent status updated: ${parsed.status}`;
                    agentId = parsed.agent_id || 'SYSTEM';
                } else if (parsed && parsed.type === 'metric') {
                    setMetrics(prev => [...prev.slice(-999), parsed]); // Keep last 1000 metrics
                    // Do not push metrics to the stdout log UI, so we return early
                    return;
                } else if (parsed && typeof parsed.log_line === 'string') {
                    logText = parsed.log_line;
                    agentId = parsed.agent_id || 'UNKNOWN';
                }
            } catch (err) {
                // Ignore parse errors, fallback to raw text
            }
            logBufferRef.current.push({ agentId, text: logText });
        };

        ws.onclose = () => {
            setConnectionStatus('disconnected');
            wsRef.current = null;
            // Auto-reconnect with a 3-second backoff
            reconnectTimeoutRef.current = setTimeout(() => {
                connect();
            }, 3000);
        };

        ws.onerror = (err) => {
            console.error('WebSocket error:', err);
            // Closing forces the onclose event, which handles the reconnect
            ws.close();
        };
    }, [url]);

    useEffect(() => {
        connect();

        // Throttling mechanism: flush the buffer 4 times a second (every 250ms)
        // This prevents massive re-renders when hundreds of logs arrive per second
        const flushInterval = setInterval(() => {
            if (logBufferRef.current.length > 0) {
                setLogs(prevLogs => {
                    const newLogs = [...prevLogs, ...logBufferRef.current];
                    // Keep array size manageable to prevent memory leaks and UI lag
                    if (newLogs.length > MAX_LOGS) {
                        return newLogs.slice(newLogs.length - MAX_LOGS);
                    }
                    return newLogs;
                });
                logBufferRef.current = []; // Clear the buffer after flushing
            }
        }, 250);

        return () => {
            // Cleanup WebSocket and timers on component unmount
            if (wsRef.current) {
                wsRef.current.onclose = null; // Prevent reconnect attempts after unmount
                wsRef.current.close();
            }
            if (reconnectTimeoutRef.current) {
                clearTimeout(reconnectTimeoutRef.current);
            }
            clearInterval(flushInterval);
        };
    }, [connect]);

    // Expose a function to send JSON commands ('start', 'stop', etc.)
    const sendCommand = useCallback((payload: CommandPayload) => {
        if (wsRef.current && wsRef.current.readyState === WebSocket.OPEN) {
            wsRef.current.send(JSON.stringify(payload));
        } else {
            console.warn("Cannot send command, WebSocket is not connected.");
        }
    }, []);

    return { connectionStatus, logs, metrics, runStatus, sendCommand };
}
