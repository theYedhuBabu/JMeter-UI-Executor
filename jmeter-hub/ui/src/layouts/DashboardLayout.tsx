import { useState } from 'react';
import { Outlet, NavLink } from 'react-router-dom';
import { Activity, Play, Upload, FileText, Settings, Menu, X } from 'lucide-react';

export function DashboardLayout() {
    const [isSidebarOpen, setIsSidebarOpen] = useState(false);

    return (
        <div className="flex h-screen bg-gray-50 flex-col md:flex-row">
            {/* Header for Mobile/Hamburger */}
            <header className="md:hidden flex items-center justify-between p-4 bg-white border-b border-gray-200">
                <div className="flex items-center gap-2">
                    <Activity className="w-6 h-6 text-blue-600" />
                    <h1 className="text-xl font-bold text-blue-600">JMeter Hub</h1>
                </div>
                <button
                    onClick={() => setIsSidebarOpen(!isSidebarOpen)}
                    className="p-2 text-gray-600 hover:bg-gray-100 rounded-md focus:outline-none"
                    aria-label="Toggle Menu"
                >
                    {isSidebarOpen ? <X className="w-6 h-6" /> : <Menu className="w-6 h-6" />}
                </button>
            </header>

            {/* Sidebar Overlay (Mobile) */}
            {isSidebarOpen && (
                <div
                    className="fixed inset-0 z-20 bg-black/50 md:hidden"
                    onClick={() => setIsSidebarOpen(false)}
                />
            )}

            {/* Sidebar Content */}
            <aside
                className={`
                    fixed inset-y-0 left-0 z-30 w-64 bg-white border-r border-gray-200 transform transition-transform duration-300 ease-in-out md:relative md:translate-x-0
                    ${isSidebarOpen ? 'translate-x-0' : '-translate-x-full'}
                `}
            >
                <div className="hidden md:flex p-4 border-b border-gray-200 items-center gap-2 h-[73px]">
                    <Activity className="w-6 h-6 text-blue-600" />
                    <h1 className="text-xl font-bold text-blue-600">JMeter Hub</h1>
                </div>

                <nav className="p-4 space-y-1">
                    <NavLink
                        to="/"
                        onClick={() => setIsSidebarOpen(false)}
                        className={({ isActive }) => `flex items-center gap-3 px-3 py-2 rounded-md ${isActive ? 'bg-blue-50 text-blue-700' : 'text-gray-700 hover:bg-gray-100'}`}
                        end
                    >
                        <Activity className="w-5 h-5" />
                        Dashboard
                    </NavLink>
                    <NavLink
                        to="/execution"
                        onClick={() => setIsSidebarOpen(false)}
                        className={({ isActive }) => `flex items-center gap-3 px-3 py-2 rounded-md ${isActive ? 'bg-blue-50 text-blue-700' : 'text-gray-700 hover:bg-gray-100'}`}
                    >
                        <Play className="w-5 h-5" />
                        Test Execution
                    </NavLink>
                    <NavLink
                        to="/upload"
                        onClick={() => setIsSidebarOpen(false)}
                        className={({ isActive }) => `flex items-center gap-3 px-3 py-2 rounded-md ${isActive ? 'bg-blue-50 text-blue-700' : 'text-gray-700 hover:bg-gray-100'}`}
                    >
                        <Upload className="w-5 h-5" />
                        Upload Scripts
                    </NavLink>
                    <NavLink
                        to="/history"
                        onClick={() => setIsSidebarOpen(false)}
                        className={({ isActive }) => `flex items-center gap-3 px-3 py-2 rounded-md ${isActive ? 'bg-blue-50 text-blue-700' : 'text-gray-700 hover:bg-gray-100'}`}
                    >
                        <FileText className="w-5 h-5" />
                        History & Reports
                    </NavLink>
                    <NavLink
                        to="/settings"
                        onClick={() => setIsSidebarOpen(false)}
                        className={({ isActive }) => `flex items-center gap-3 px-3 py-2 rounded-md ${isActive ? 'bg-blue-50 text-blue-700' : 'text-gray-700 hover:bg-gray-100'}`}
                    >
                        <Settings className="w-5 h-5" />
                        Settings
                    </NavLink>
                </nav>
            </aside>

            {/* Main Content Area */}
            <main className="flex-1 p-4 md:p-8 overflow-auto w-full">
                <Outlet />
            </main>
        </div>
    );
}
