import React from 'react'
import { Navigate, Route, Routes, useLocation } from 'react-router-dom'

import { EditorPage } from './pages/EditorPage'
import { DashboardPage } from './pages/DashboardPage'
import { SettingsPage } from './pages/SettingsPage'
import { SetPasswordPage } from './pages/SetPasswordPage'
import { LoginPage } from './pages/LoginPage'
import axios from 'axios'
import { useProfileQuery } from './common/api/auth'

axios.defaults.withCredentials = true

const AuthRequired: React.FC<{ children: React.ReactNode }> = ({
  children,
}) => {
  const { data, isLoading } = useProfileQuery()
  const { pathname } = useLocation()

  if (isLoading || data == null) return null

  if (!data?.isPasswordSet) return <Navigate to="/welcome" replace />
  if (data?.isPasswordSet && pathname == '/welcome')
    return <Navigate to="/" replace />

  if (!data.isLoggedIn) return <Navigate to="/login" replace />
  if (data?.isLoggedIn && pathname == '/login')
    return <Navigate to="/" replace />

  return <>{children}</>
}

const App: React.FC = () => {
  return (
    <Routes>
      <Route
        path="/"
        element={
          <AuthRequired>
            <DashboardPage />
          </AuthRequired>
        }
      />
      <Route
        path="/settings"
        element={
          <AuthRequired>
            <SettingsPage />
          </AuthRequired>
        }
      />
      <Route
        path="/editor"
        element={
          <AuthRequired>
            <EditorPage />
          </AuthRequired>
        }
      />
      <Route path="/login" element={<LoginPage />} />
      <Route path="/welcome" element={<SetPasswordPage />} />
    </Routes>
  )
}

export default App
