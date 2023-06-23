import React, { useState } from 'react'
import { MdOutlineKey } from 'react-icons/md'
import { Navigate } from 'react-router-dom'
import { useLoginMutation, useProfileQuery } from '../common/api/auth'

export const LoginPage: React.FC = () => {
  const viewer = useProfileQuery()
  const [password, setPassword] = useState('')

  const { mutate, isError, error } = useLoginMutation()

  if (viewer.data?.isLoggedIn) return <Navigate to="/" replace />
  console.log(viewer.data)
  return (
    <div className="container mx-auto max-w-2xl pt-16">
      <form
        onSubmit={(e) => {
          e.preventDefault()
          mutate(password)
        }}
        className="border rounded-xl border-neutral-600 p-20"
      >
        <h1 className="font-serif text-4xl">Weights &amp; Biases Console</h1>

        <p className="mt-4 text-xl font-thin text-neutral-300">
          Login using your root password.
        </p>

        <input
          value={password}
          onChange={(e) => setPassword(e.target.value)}
          type="password"
          placeholder="Enter password"
          className="mt-8 w-full rounded-md bg-neutral-700 p-2 px-4 text-lg placeholder:text-neutral-400"
        />

        <button
          type="submit"
          className="mt-6 text-md flex items-center rounded-md bg-yellow-400 p-3 px-5 text-lg font-semibold text-black disabled:bg-neutral-400"
        >
          <MdOutlineKey className="mr-2" />
          Login
        </button>

        {isError && error != null && (
          <div className="text-red-300 mt-4">
            {(error as any).response.data.error}
          </div>
        )}
      </form>
    </div>
  )
}
