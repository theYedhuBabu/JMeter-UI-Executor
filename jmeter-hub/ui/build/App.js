"use strict";
Object.defineProperty(exports, "__esModule", { value: true });
const react_router_dom_1 = require("react-router-dom");
const DashboardLayout_1 = require("./layouts/DashboardLayout");
function App() {
    return (<react_router_dom_1.BrowserRouter>
      <react_router_dom_1.Routes>
        <react_router_dom_1.Route path="/" element={<DashboardLayout_1.DashboardLayout />}>
          <react_router_dom_1.Route index element={<div className="bg-white p-6 rounded-xl shadow-sm border border-gray-100 h-full">
              <h3 className="text-xl font-semibold mb-4 text-gray-800">System Overview</h3>
              <p className="text-gray-600">Welcome to JMeter Hub. Select an option from the sidebar to manage test execution, scripts, and results.</p>
            </div>}/>
          <react_router_dom_1.Route path="execution" element={<div className="p-4 bg-white rounded shadow">Test Execution Module</div>}/>
          <react_router_dom_1.Route path="upload" element={<div className="p-4 bg-white rounded shadow">Script Upload Module</div>}/>
          <react_router_dom_1.Route path="history" element={<div className="p-4 bg-white rounded shadow">History & Reports</div>}/>
          <react_router_dom_1.Route path="settings" element={<div className="p-4 bg-white rounded shadow">Agent Settings</div>}/>
        </react_router_dom_1.Route>
      </react_router_dom_1.Routes>
    </react_router_dom_1.BrowserRouter>);
}
exports.default = App;
