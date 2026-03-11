import { BrowserRouter as Router, Routes, Route } from 'react-router-dom';
import { DashboardLayout } from './layouts/DashboardLayout';
import { HistoryTable } from './components/HistoryTable';
import { TestRunner } from './components/TestRunner';
import {DashboardPage} from './pages/Dashboard'

function App() {
  return (
    <Router>
      <Routes>
        <Route path="/" element={<DashboardLayout />}>
          <Route index element={<DashboardPage />} />
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
