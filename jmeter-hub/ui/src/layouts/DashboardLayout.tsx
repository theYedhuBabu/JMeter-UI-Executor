import { Outlet, NavLink } from 'react-router-dom';
import { Activity, Play, Upload, FileText, Settings } from 'lucide-react';

export function DashboardLayout() {
    return (
        <div className="flex h-screen bg-gray-50">
            <aside className="w-64 bg-white border-r border-gray-200">
                <div className="p-4 border-b border-gray-200">
                    <h1 className="text-xl font-bold text-blue-600 flex items-center gap-2">
                        <Activity className="w-6 h-6" />
                        JMeter Hub
                    </h1>
                </div>
                <nav className="p-4 space-y-1">
                    <NavLink to="/" className={({ isActive }) => `flex items-center gap-3 px-3 py-2 rounded-md ${isActive ? 'bg-blue-50 text-blue-700' : 'text-gray-700 hover:bg-gray-100'}`} end>
                        <Activity className="w-5 h-5" />
                        Dashboard
                    </NavLink>
                    <NavLink to="/execution" className={({ isActive }) => `flex items-center gap-3 px-3 py-2 rounded-md ${isActive ? 'bg-blue-50 text-blue-700' : 'text-gray-700 hover:bg-gray-100'}`}>
                        <Play className="w-5 h-5" />
                        Test Execution
                    </NavLink>
                    <NavLink to="/upload" className={({ isActive }) => `flex items-center gap-3 px-3 py-2 rounded-md ${isActive ? 'bg-blue-50 text-blue-700' : 'text-gray-700 hover:bg-gray-100'}`}>
                        <Upload className="w-5 h-5" />
                        Upload Scripts
                    </NavLink>
                    <NavLink to="/history" className={({ isActive }) => `flex items-center gap-3 px-3 py-2 rounded-md ${isActive ? 'bg-blue-50 text-blue-700' : 'text-gray-700 hover:bg-gray-100'}`}>
                        <FileText className="w-5 h-5" />
                        History & Reports
                    </NavLink>
                    <NavLink to="/settings" className={({ isActive }) => `flex items-center gap-3 px-3 py-2 rounded-md ${isActive ? 'bg-blue-50 text-blue-700' : 'text-gray-700 hover:bg-gray-100'}`}>
                        <Settings className="w-5 h-5" />
                        Settings
                    </NavLink>
                </nav>
            </aside>
            <main className="flex-1 p-8 overflow-auto">
                <Outlet />
            </main>
        </div>
    );
}
