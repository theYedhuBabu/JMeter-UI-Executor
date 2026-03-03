import { BrowserRouter as Router, Routes, Route } from 'react-router-dom';
import { DashboardLayout } from './layouts/DashboardLayout';
import { HistoryTable } from './components/HistoryTable';
import { TestRunner } from './components/TestRunner';

function App() {
  return (
    <Router>
      <Routes>
        <Route path="/" element={<DashboardLayout />}>
          <Route index element={
            <div className="bg-white p-6 rounded-xl shadow-sm border border-gray-100 h-full">
              <h3 className="text-xl font-semibold mb-4 text-gray-800">System Overview</h3>
              <p className="text-gray-600">Welcome to JMeter Hub. Select an option from the sidebar to manage test execution, scripts, and results.</p>
            </div>
          } />
          <Route path="execution" element={<TestRunner />} />
          <Route path="upload" element={<div className="p-4 bg-white rounded shadow">Script Upload Module</div>} />
          <Route path="history" element={<HistoryTable />} />
          <Route path="settings" element={<div className="p-4 bg-white rounded shadow">Agent Settings</div>} />
        </Route>
      </Routes>
    </Router>
  );
}

export default App;
