import React from 'react'
import { Route, Routes } from 'react-router-dom'

import { EditorPage } from './pages/EditorPage'
import { DashboardPage } from './pages/DashboardPage'
import { SettingsPage } from './pages/SettingsPage'
import { SetPasswordPage } from './pages/SetPasswordPage'

const App: React.FC = () => {
  return (
    <Routes>
      <Route path="/" element={<DashboardPage />} />
      <Route path="/settings" element={<SettingsPage />} />
      <Route path="/editor" element={<EditorPage />} />
      <Route path="/password" element={<SetPasswordPage />} />
    </Routes>
  )
}

export default App
