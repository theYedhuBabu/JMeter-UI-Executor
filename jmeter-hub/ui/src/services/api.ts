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

async function apiFetch<T>(url:string, options?: RequestInit): Promise<T> {

    const response = await fetch(`${API_BASE_URL}${url}`, options);

    if (!response.ok) {
        throw new Error(`${response.status}: ${response.statusText}`)
    }
    return response.json();
    
}

export async function parseJmx(file: File): Promise<string[]> {
    const formData = new FormData();
    formData.append('file', file);

    const result = await apiFetch<{required_csvs : string[]}>("/api/parse-jmx", {
        method: 'POST',
        body: formData,
    });
    return result.required_csvs || [];
}

export async function processFramework() {
    
}

export async function uploadScript(file: File, csvFiles: Record<string, File>, configStr: string): Promise<any> {
    const formData = new FormData();
    formData.append('file', file);

    Object.entries(csvFiles).forEach(([key, csv]) => {
        formData.append(key, csv);
    });

    formData.append('config', configStr);

    return apiFetch('/api/upload/script', {
        method: 'POST',
        body: formData,
    });
}

export async function getHistory(): Promise<TestRun[]> {
    const result = await apiFetch<{ data: TestRun[] }>('/api/history');
    return result.data || [];
}

export async function getAgents(): Promise<string[]> {
    const result = await apiFetch<{ agents: string[] }>('/api/agents');
    return result.agents || [];
}

export async function getActiveRun(): Promise<{ active: boolean; runId: string | null }> {
    return apiFetch('/api/run/active');
}
