import React, { useState } from 'react'
import { MdOutlineKey } from 'react-icons/md'
import { Navigate } from 'react-router-dom'
import { useProfileQuery, usePasswordMutation } from '../common/api/auth'

export const SetPasswordPage: React.FC = () => {
  const viewer = useProfileQuery()
  const { mutate, isError, error } = usePasswordMutation()
  const [password, setPassword] = useState('')
  if (viewer.data?.isPasswordSet) return <Navigate to="/" replace />

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
          The root password serves as the key to log in to the console. In case
          you forget this password, you will have to reset it directly by
          establishing a connection with the Kubernetes cluster.
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
          Set Password
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
