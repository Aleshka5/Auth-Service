import { BrowserRouter, Route, Routes } from 'react-router-dom'
import { AuthProvider } from './auth'
import { Layout } from './components/Layout'
import { LegacyArchitectureRedirect, NodeRoute } from './components/tree/NodeRoute'
import { Admin } from './pages/Admin'
import { Auth } from './pages/Auth'

export default function App() {
  return (
    <BrowserRouter>
      <AuthProvider>
        <Routes>
          <Route element={<Layout />}>
            <Route path="/auth" element={<Auth />} />
            <Route path="/admin" element={<Admin />} />
            <Route path="/architecture/:slug" element={<LegacyArchitectureRedirect />} />
            <Route path="*" element={<NodeRoute />} />
          </Route>
        </Routes>
      </AuthProvider>
    </BrowserRouter>
  )
}
