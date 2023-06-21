import React from 'react'
import { MdOutlineKey } from 'react-icons/md'

export const SetPasswordPage: React.FC = () => {
  return (
    <div className="container mx-auto max-w-2xl pt-16">
      <div className="border rounded-xl border-neutral-600 p-20">
        <h1 className="font-serif text-4xl">Weights &amp; Biases Console</h1>

        <p className="mt-4 text-xl font-thin text-neutral-300">
          The root password serves as the key to log in to the console. In case
          you forget this password, you will have to reset it directly by
          establishing a connection with the Kubernetes cluster.
        </p>

        <input
          value=""
          placeholder="Enter password"
          className="mt-8 w-full rounded-md bg-neutral-700 p-2 px-4 text-lg placeholder:text-neutral-400"
        />

        <button className="mt-6 text-md flex items-center rounded-md bg-yellow-400 p-3 px-5 text-lg font-semibold text-black disabled:bg-neutral-400">
          <MdOutlineKey className="mr-2" />
          Set Password
        </button>
      </div>
    </div>
  )
}
