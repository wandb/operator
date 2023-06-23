import React, { useEffect, useState } from 'react'
import { TopNavbar } from '../common/TopNavbar'
import { MdWarning } from 'react-icons/md'
import {
  useLatestConfigMutation,
  useLatestConfigQuery,
} from '../common/api/config'

const LicenseCard: React.FC = () => {
  const { data } = useLatestConfigQuery()
  const { mutate } = useLatestConfigMutation()

  const [license, setLicense] = useState<string>('')
  useEffect(() => setLicense(data?.config.license), [data])
  return (
    <form
      onSubmit={(e) => {
        console.log({ ...data, license })
        e.preventDefault()
        if (license === data?.config.license) return
        mutate({ config: { ...data?.config, license } })
      }}
    >
      <h2 className="text-neutral-300 text-xl">License</h2>
      <p className="text-neutral-400 mt-1">
        Configure and check license status.
      </p>
      <div className="rounded-md bg-neutral-800 p-6 my-4 grid gap-2">
        <textarea
          value={license}
          onChange={(e) => setLicense(e.target.value)}
          className="border border-neutral-700 text-white rounded-md bg-transparent placeholder:text-neutral-500 px-2 py-1"
        />
      </div>
      <button className="text-black font-semibold hover:bg-green-400 rounded-md py-2 bg-green-500 px-4">
        Save
      </button>
    </form>
  )
}

type BucketConfig = {
  connectionString: string
  region?: string
  kmsKey?: string
}
const BucketCard: React.FC<{
  value: BucketConfig
  onChange: (value: BucketConfig) => void
}> = ({ value, onChange }) => {
  return (
    <form>
      <h2 className="text-neutral-300 text-xl">Bucket</h2>
      <p className="text-neutral-400 mt-1">
        Connection settings for the S3 compatible storage. Note, changing the
        Bucket connection will not migrate the data automatically.
      </p>
      <div className="rounded-md bg-neutral-800 p-6 my-4 grid gap-4">
        <div className="flex items-center gap-4">
          <div className="flex-grow max-w-[300px]">
            <label
              className="block text-neutral-300 text-sm font-bold mb-1"
              htmlFor="host"
            >
              Bucket
            </label>
            <input
              id="host"
              value={value.connectionString}
              onChange={(e) =>
                onChange({ ...value, connectionString: e.target.value })
              }
              className="w-full border border-neutral-700 rounded-md bg-transparent placeholder:text-neutral-500 px-2 py-1"
              placeholder="host"
            />
          </div>
          <div className="flex-grow max-w-[300px]">
            <label
              className="block text-neutral-300 text-sm font-bold mb-1"
              htmlFor="region"
            >
              AWS Region
            </label>
            <input
              id="region"
              value={value.region}
              onChange={(e) =>
                onChange({ ...value, connectionString: e.target.value })
              }
              className="w-full border border-neutral-700 rounded-md bg-transparent placeholder:text-neutral-500 px-2 py-1"
              placeholder="AWS Region"
            />
          </div>
        </div>
        <div className="flex-grow max-w-[300px]">
          <label
            className="block text-neutral-300 text-sm font-bold mb-1"
            htmlFor="kmsKey"
          >
            KMS Key ARN
          </label>
          <input
            id="kmsKey"
            value={value.kmsKey}
            onChange={(e) =>
              onChange({ ...value, connectionString: e.target.value })
            }
            className="w-full border border-neutral-700 rounded-md bg-transparent placeholder:text-neutral-500 px-2 py-1"
            placeholder="KMS Key ARN"
          />
        </div>
      </div>
      <button
        type="submit"
        className="text-black font-semibold hover:bg-green-400 rounded-md py-2 bg-green-500 px-4"
      >
        Save
      </button>
    </form>
  )
}

type MysqlConfig = {
  host: string
  port: number
  database: string
  username: string
  password: string
}

const DatabaseCard: React.FC = () => {
  const { data } = useLatestConfigQuery()
  const { mutate } = useLatestConfigMutation()
  const database = data?.config.mysql
  const [config, setConfig] = useState<MysqlConfig>({
    database: '',
    host: '',
    password: '',
    port: 3306,
    username: '',
  })
  const [isExternal, setExternal] = useState(database != null)
  return (
    <form
      onSubmit={() => mutate({ config: { ...data?.config, mysql: config } })}
    >
      <h2 className="text-neutral-300 text-xl">MySQL</h2>
      <p className="text-neutral-400 mt-1">
        Connection settings for the MySQL database. Note, changing the MySQL
        connection will not migrate the data automatically to the new instance.
      </p>

      {database == null && (
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

      <div className="rounded-md bg-neutral-800 p-6">
        {isExternal ? (
          <div>
            <div className="text-neutral-300">Connection</div>
            <div className="flex items-center gap-4">
              <div className="flex-grow max-w-[300px]">
                <label
                  className="block text-neutral-300 text-sm font-bold mb-1"
                  htmlFor="host"
                >
                  Host
                </label>
                <input
                  id="host"
                  value={config?.host}
                  onChange={(e) =>
                    setConfig({ ...config, host: e.target.value })
                  }
                  className="w-full border border-neutral-700 rounded-md bg-transparent placeholder:text-neutral-500 px-2 py-1"
                  placeholder="host"
                />
              </div>
              <div>
                <label
                  className="block text-neutral-300 text-sm font-bold mb-1"
                  htmlFor="database"
                >
                  Database
                </label>
                <input
                  value={config?.database}
                  onChange={(e) =>
                    setConfig({ ...config, database: e.target.value })
                  }
                  className="w-40 border border-neutral-700 rounded-md bg-transparent placeholder:text-neutral-500 px-2 py-1"
                  placeholder="Database"
                  id="database"
                />
              </div>
              <div>
                <label
                  className="block text-neutral-300 text-sm font-bold mb-1"
                  htmlFor="port"
                >
                  Port
                </label>
                <input
                  type="number"
                  value={config?.port}
                  onChange={(e) =>
                    setConfig({ ...config, port: e.target.valueAsNumber })
                  }
                  className="w-20 border border-neutral-700 rounded-md bg-transparent placeholder:text-neutral-500 px-2 py-1"
                  placeholder="port"
                  id="port"
                />
              </div>
            </div>
            <div className="mt-2 text-neutral-300">Authentication</div>
            <div className="items-center flex gap-4">
              <div>
                <label
                  className="block text-neutral-300 text-sm font-bold mb-1"
                  htmlFor="port"
                >
                  Username
                </label>
                <input
                  value={config?.username}
                  onChange={(e) =>
                    setConfig({ ...config, username: e.target.value })
                  }
                  className="border border-neutral-700 rounded-md bg-transparent placeholder:text-neutral-500 px-2 py-1"
                  placeholder="username"
                  id="username"
                />
              </div>
              <div>
                <label
                  className="block text-neutral-300 text-sm font-bold mb-1"
                  htmlFor="port"
                >
                  Password
                </label>
                <input
                  value={config?.password}
                  onChange={(e) =>
                    setConfig({ ...config, password: e.target.value })
                  }
                  className="border border-neutral-700 rounded-md bg-transparent placeholder:text-neutral-500 px-2 py-1"
                  placeholder="password"
                  type="Password"
                />
              </div>
            </div>
          </div>
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

      <button
        type="submit"
        className="mt-4 text-black font-semibold hover:bg-green-400 rounded-md py-2 bg-green-500 px-4"
      >
        Save
      </button>
    </form>
  )
}

export const SettingsPage: React.FC = () => {
  const [config, setConfig] = useState<any>({ bucket: {} })
  const { data, isLoading } = useLatestConfigQuery()
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
      <div className="max-w-5xl mx-auto mt-10 mb-20">
        <h1 className="text-3xl font-semibold tracking-wide mb-4">Settings</h1>

        <div className="grid gap-8 my-8 max-w-[650px]">
          <LicenseCard />
          <BucketCard
            value={config.bucket}
            onChange={(bucket) => {
              setConfig((c: any) => ({ ...c, bucket }))
            }}
          />
          <DatabaseCard />
        </div>
      </div>
    </>
  )
}
