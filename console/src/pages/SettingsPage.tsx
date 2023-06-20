import React, { useEffect, useState } from 'react'
import { TopNavbar } from '../common/TopNavbar'
import { useConfigQuery } from '../common/api'
import { MdWarning } from 'react-icons/md'

type BucketConfig = {
  connectionString: string
  region?: string
  kmsKey?: string
}
const BucketCard: React.FC<{
  value: BucketConfig
  onChange: (value: BucketConfig) => void
}> = ({ value }) => {
  const isS3 = value.connectionString?.startsWith('s3://')
  return (
    <div>
      <h2 className="text-neutral-300 text-xl">Bucket</h2>
      <div>
        <input
          className="px-2 text-lg bg-neutral-700 rounded-md"
          value={value.connectionString}
        />
        {isS3 && (
          <input
            className="text-lg bg-neutral-700 rounded-md"
            value={value.region}
          />
        )}
      </div>
      <button className="mt-4 text-black font-semibold hover:bg-green-400 rounded-md py-2 bg-green-500 px-4">
        Save
      </button>
    </div>
  )
}

type MysqlConfig = {
  host: string
  port: string
  database: string
  username: string
  password: string
}

const DatabaseCard: React.FC<{
  value?: MysqlConfig
  onChange?: (v: MysqlConfig) => void
}> = ({ value, onChange }) => {
  const [config, setConfig] = useState<MysqlConfig>(
    value != null
      ? value
      : { database: '', host: '', password: '', port: '', username: '' },
  )
  const [isExternal, setExternal] = useState(value != null)
  return (
    <div>
      <h2 className="text-neutral-300 text-xl">MySQL</h2>
      {value == null && (
        <div className="inline-block items-center border border-neutral-700 rounded-lg p-1 my-4">
          <button
            className={`rounded-md px-2 py-1 mr-2 ${
              isExternal && 'bg-blue-600'
            }`}
            onClick={() => setExternal(true)}
          >
            External
          </button>
          <button
            className={`rounded-md px-2 py-1 mr-2 ${
              !isExternal && 'bg-red-600'
            }`}
            onClick={() => setExternal(false)}
          >
            Internal
          </button>
        </div>
      )}

      <div className="rounded-md bg-neutral-800 p-4">
        {isExternal ? (
          <form
            onSubmit={(e) => {
              onChange?.(config)
              e.preventDefault()
            }}
          >
            <div>
              <input
                value={config?.host}
                onChange={(e) => setConfig({ ...config, host: e.target.value })}
                className="border border-neutral-700 rounded-md bg-transparent placeholder:text-neutral-500 px-2 py-1"
                placeholder="host"
              />
            </div>
            <div>
              <input
                value={config?.database}
                onChange={(e) =>
                  setConfig({ ...config, database: e.target.value })
                }
                className="border border-neutral-700 rounded-md bg-transparent placeholder:text-neutral-500 px-2 py-1"
                placeholder="database"
              />
            </div>
            <div>
              <input
                value={config?.port}
                onChange={(e) => setConfig({ ...config, port: e.target.value })}
                className="border border-neutral-700 rounded-md bg-transparent placeholder:text-neutral-500 px-2 py-1"
                placeholder="port"
              />
            </div>
            <div>
              <input
                value={config?.username}
                onChange={(e) =>
                  setConfig({ ...config, username: e.target.value })
                }
                className="border border-neutral-700 rounded-md bg-transparent placeholder:text-neutral-500 px-2 py-1"
                placeholder="username"
              />
            </div>
            <div>
              <input
                value={config?.password}
                onChange={(e) =>
                  setConfig({ ...config, password: e.target.value })
                }
                className="border border-neutral-700 rounded-md bg-transparent placeholder:text-neutral-500 px-2 py-1"
                placeholder="password"
                type="password"
              />
            </div>
          </form>
        ) : (
          <div className="text-center">
            <p className="text-red-400 text-lg">
              <MdWarning className="inline-block mr-2 w-4 mb-1" />
              Please provide an external database.
            </p>
            <p className="text-neutral-300">
              We have deployed a database for you. However, this database is not
              persistent.
            </p>
          </div>
        )}
      </div>

      <button className="mt-4 text-black font-semibold hover:bg-green-400 rounded-md py-2 bg-green-500 px-4">
        Save
      </button>
    </div>
  )
}

export const SettingsPage: React.FC = () => {
  const [config, setConfig] = useState<any>({ bucket: {} })
  const { data, isLoading } = useConfigQuery()
  useEffect(() => {
    if (data == null) return
    setConfig(data)
  }, [config, setConfig, data])

  if (isLoading) {
    return null
  }

  return (
    <>
      <TopNavbar />
      <div className="max-w-5xl mx-auto mt-10">
        <h1 className="text-3xl font-semibold tracking-wide mb-4">Settings</h1>

        <div className="grid gap-8 mt-8">
          <BucketCard
            value={config.bucket}
            onChange={(bucket) => {
              setConfig((c: any) => ({ ...c, bucket }))
            }}
          />
          <DatabaseCard />
          <h2 className="text-neutral-300 text-xl mt-6">Authentication</h2>
        </div>
      </div>
    </>
  )
}
