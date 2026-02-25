import { BrowserRouter, Routes, Route } from 'react-router-dom';
import { Layout } from './components/Layout';
import { BrowseList } from './components/BrowseList';
import { DetailView } from './components/DetailView';
import { Dashboard } from './components/Dashboard';

export function App() {
  return (
    <BrowserRouter>
      <Routes>
        <Route element={<Layout />}>
          <Route path="/" element={<BrowseList />} />
          <Route path="/wanted/:id" element={<DetailView />} />
          <Route path="/me" element={<Dashboard />} />
        </Route>
      </Routes>
    </BrowserRouter>
  );
}
