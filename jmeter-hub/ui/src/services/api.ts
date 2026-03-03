const API_BASE_URL = 'http://localhost:8080';

export interface TestRun {
    ID: number;
    ScriptName: string;
    Status: string;
    StartTime: string;
    EndTime: string;
    LogPath: string;
}

export async function uploadScript(file: File, csvFiles: File[]): Promise<any> {
    const formData = new FormData();
    formData.append('file', file);
    csvFiles.forEach((csv) => {
        formData.append('file', csv);
    });

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
