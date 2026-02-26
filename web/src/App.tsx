import { BrowserRouter, Route, Routes } from "react-router-dom";
import { BrowseList } from "./components/BrowseList";
import { ConnectPage } from "./components/ConnectPage";
import { Dashboard } from "./components/Dashboard";
import { DetailView } from "./components/DetailView";
import { Layout } from "./components/Layout";
import { Settings } from "./components/Settings";

export function App() {
  return (
    <BrowserRouter>
      <Routes>
        <Route element={<Layout />}>
          <Route path="/" element={<BrowseList />} />
          <Route path="/wanted/:id" element={<DetailView />} />
          <Route path="/me" element={<Dashboard />} />
          <Route path="/settings" element={<Settings />} />
          <Route path="/connect" element={<ConnectPage />} />
        </Route>
      </Routes>
    </BrowserRouter>
  );
}
