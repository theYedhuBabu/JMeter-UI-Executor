// In development, fall back to localhost:8080. In production, use the host that served the page.
const API_BASE_URL = import.meta.env.DEV ? 'http://localhost:8080' : '';
export interface TestRun {
    ID: string;
    ScriptName: string;
    Status: string;
    StartTime: string;
    EndTime: string;
    LogPath: string;
}

export async function parseJmx(file: File): Promise<string[]> {
    const formData = new FormData();
    formData.append('file', file);

    const response = await fetch(`${API_BASE_URL}/api/parse-jmx`, {
        method: 'POST',
        body: formData,
    });

    if (!response.ok) {
        throw new Error(`Failed to parse JMX: ${response.statusText}`);
    }

    const result = await response.json();
    return result.required_csvs || [];
}

export async function uploadScript(file: File, csvFiles: Record<string, File>, configStr: string): Promise<any> {
    const formData = new FormData();
    formData.append('file', file);

    // Append each csv file under its specific variable name key
    Object.entries(csvFiles).forEach(([key, csv]) => {
        formData.append(key, csv);
    });

    // Append the json config
    formData.append('config', configStr);

    const response = await fetch(`${API_BASE_URL}/api/upload/script`, {
        method: 'POST',
        body: formData,
    });

    if (!response.ok) {
        throw new Error(`Failed to upload script: ${response.statusText}`);
    }

    return response.json();
}

export async function getHistory(): Promise<TestRun[]> {
    const response = await fetch(`${API_BASE_URL}/api/history`);
    if (!response.ok) {
        throw new Error(`Failed to fetch history: ${response.statusText}`);
    }
    const result = await response.json();
    return result.data || [];
}

export async function getAgents(): Promise<string[]> {
    const response = await fetch(`${API_BASE_URL}/api/agents`);
    if (!response.ok) {
        throw new Error(`Failed to fetch agents: ${response.statusText}`);
    }
    const result = await response.json();
    return result.agents || [];
}

export async function getActiveRun(): Promise<{ active: boolean; runId: string | null }> {
    const response = await fetch(`${API_BASE_URL}/api/run/active`);
    if (!response.ok) {
        throw new Error(`Failed to fetch active run: ${response.statusText}`);
    }
    return response.json();
}
