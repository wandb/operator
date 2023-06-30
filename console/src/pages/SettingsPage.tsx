import React, { useEffect, useState } from 'react'
import { TopNavbar } from '../common/TopNavbar'
import { MdCheckCircleOutline, MdPending, MdWarning } from 'react-icons/md'
import {
  useLatestConfigMutation,
  useLatestConfigQuery,
} from '../common/api/config'
import { isEqual } from 'lodash'
import { usePodsQuery } from '../common/api/k8s'
import { LogDialog } from '../common/PodLogs'

const LicenseCard: React.FC = () => {
  const { data } = useLatestConfigQuery()
  const { mutate } = useLatestConfigMutation()

  const [license, setLicense] = useState<string>('')
  useEffect(() => setLicense(data?.config.license), [data])

  const hasChanged = !isEqual(data?.config.license, license)

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
      <button
        disabled={!hasChanged}
        className="text-black font-semibold hover:bg-green-400 rounded-md py-2 bg-green-500 px-4 disabled:opacity-50"
      >
        Save
      </button>
    </form>
  )
}

const BucketCard: React.FC = () => {
  const { data } = useLatestConfigQuery()
  const config = data?.config ?? {}
  const [value, setValue] = useState<{
    connectionString?: string
    region?: string
    kmsKey?: string
  }>(config.bucket ?? {})
  useEffect(() => setValue(data?.config.bucket ?? {}), [data])

  const hasChange = !isEqual(data?.config.bucket, value)

  const { mutate } = useLatestConfigMutation()

  return (
    <form
      onSubmit={(e) => {
        e.preventDefault()
        mutate({ ...data, config: { ...data?.config, bucket: value } })
      }}
    >
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
                setValue({ ...value, connectionString: e.target.value })
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
              onChange={(e) => setValue({ ...value, region: e.target.value })}
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
            onChange={(e) => setValue({ ...value, kmsKey: e.target.value })}
            className="w-full border border-neutral-700 rounded-md bg-transparent placeholder:text-neutral-500 px-2 py-1"
            placeholder="KMS Key ARN"
          />
        </div>
      </div>
      <button
        disabled={!hasChange}
        type="submit"
        className="text-black font-semibold hover:bg-green-400 rounded-md py-2 bg-green-500 px-4 disabled:opacity-50"
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

      <div className="rounded-md bg-neutral-800 p-6 my-4">
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

type SsoConfig = {
  ldap?: any
  oidc?: {
    clientId: string
    issuer: string
    method: string
  }
}

const SsoCard: React.FC = () => {
  const { data } = useLatestConfigQuery()
  const [value, setValue] = useState<SsoConfig>(data?.config.sso ?? {})
  useEffect(() => setValue(data?.config.sso ?? {}), [data])

  const isLdap = value.ldap != null
  const isOidc = value.oidc != null
  const isBasic = !isLdap && !isOidc

  const hasChange = !isEqual(data?.config.sso, value)

  // const ldapGroupSyncOnly = isLdap && isOidc

  const { mutate } = useLatestConfigMutation()

  return (
    <form
      onSubmit={(e) => {
        e.preventDefault()
        mutate({ ...data, config: { ...data?.config, mysql: value } })
      }}
    >
      <h2 className="text-neutral-300 text-xl">Single Sign On</h2>
      <p className="text-neutral-400 mt-1">
        Allows users to access Weights &amp; Biases or using only one set of
        login credentials.
      </p>

      <div className="inline-block items-center border border-neutral-700 rounded-lg p-1 my-4">
        <button
          type="button"
          className={`rounded-md px-2 py-1 mr-2 ${isBasic && 'bg-blue-600'}`}
          onClick={() => setValue({})}
        >
          Built in Authentication
        </button>
        <button
          type="button"
          className={`rounded-md px-2 py-1 mr-2 ${isOidc && 'bg-blue-600'}`}
          onClick={() =>
            setValue({
              oidc: {
                clientId: '',
                issuer: '',
                method: 'implicit',
              },
            })
          }
        >
          OIDC
        </button>
        <button
          type="button"
          className={`rounded-md px-2 py-1 ${isLdap && 'bg-blue-600'}`}
          onClick={() => setValue({ ldap: {} })}
        >
          LDAP
        </button>
      </div>

      <div className="rounded-md bg-neutral-800 p-6">
        {isBasic && (
          <div>Your instance is using the built in authentication system.</div>
        )}
        {isLdap && (
          <div>
            LDAP is in testing. Please contact support to get instructions on
            how to enable this feature.
          </div>
        )}
        {isOidc && (
          <div className="grid gap-3">
            <div className="flex items-center gap-3">
              <div className="flex-grow">
                <label
                  className="block text-neutral-300 text-sm font-bold mb-1"
                  htmlFor="clientid"
                >
                  Client ID
                </label>
                <input
                  id="clientid"
                  value={value?.oidc?.clientId}
                  onChange={(e) =>
                    setValue({
                      oidc: { ...value.oidc!, clientId: e.target.value },
                    })
                  }
                  className="w-full border border-neutral-700 rounded-md bg-transparent placeholder:text-neutral-500 px-2 py-1"
                  placeholder="Client ID"
                />
              </div>
              <div className="flex-grow">
                <label
                  className="block text-neutral-300 text-sm font-bold mb-1"
                  htmlFor="issuer"
                >
                  Issuer
                </label>
                <input
                  id="method"
                  value={value?.oidc?.issuer}
                  onChange={(e) =>
                    setValue({
                      oidc: { ...value.oidc!, issuer: e.target.value },
                    })
                  }
                  className="w-full border border-neutral-700 rounded-md bg-transparent placeholder:text-neutral-500 px-2 py-1"
                  placeholder="Issuer"
                />
              </div>
            </div>
            <div>
              <label
                className="block text-neutral-300 text-sm font-bold mb-1"
                htmlFor="method"
              >
                Method
              </label>
              <select
                id="method"
                onChange={(e) =>
                  setValue({
                    oidc: { ...value.oidc!, method: e.target.value },
                  })
                }
                className="w-[200px] border border-neutral-700 rounded-md bg-transparent placeholder:text-neutral-500 px-2 py-1"
              >
                <option selected value="implict">
                  Implicit Form Post
                </option>
                <option value="pkce">PKCE</option>
              </select>
            </div>
          </div>
        )}
      </div>

      <button
        type="submit"
        disabled={hasChange}
        className="mt-4 text-black font-semibold hover:bg-green-400 rounded-md py-2 bg-green-500 px-4 disabled:opacity-50"
      >
        Save
      </button>
    </form>
  )
}

const UnreadyPodRow: React.FC<{ metadata: { name: string } }> = ({
  metadata,
}) => {
  const [isOpen, setIsOpen] = useState(false)
  return (
    <>
      <div
        key={metadata.name}
        className="flex items-center space-x-2 text-neutral-400"
      >
        <div className="truncate w-0 flex-1">
          {metadata.name.replace('wandb-', '')}
        </div>
        <button
          onClick={() => setIsOpen(true)}
          className="px-2 rounded-md bg-neutral-800"
        >
          logs
        </button>
      </div>
      <LogDialog
        pod={metadata.name}
        isOpen={isOpen}
        onClose={() => setIsOpen(false)}
      />
    </>
  )
}

const StatusCard: React.FC = () => {
  const pods = usePodsQuery('default', { refetchInterval: 1000 })

  const unheathyPods = pods.data?.items?.filter(
    (pod) => pod.status.phase !== 'Running',
  )
  const allHeathy = unheathyPods?.length === 0

  const unreadyPods = pods.data?.items?.filter(({ status }) =>
    status.containerStatuses.some((p) => !p.ready),
  )
  const allReady = unreadyPods?.length === 0

  const podsHasRestarts = pods.data?.items?.filter(
    ({ status }) =>
      status.containerStatuses.some((p) => p.restartCount > 0) ||
      status.initContainerStatuses?.some((p) => p.restartCount > 0),
  )
  const hasAnyRestarts = (podsHasRestarts?.length ?? 0) > 0

  const isApplying = !allHeathy && !allReady
  const isFailling = hasAnyRestarts && !allReady
  const isNotReady = allHeathy && !allReady && !hasAnyRestarts
  const isHeathy = allHeathy && allReady

  const [showLogs, setShowLogs] = useState(false)

  return (
    <div className="sticky top-20 flex-grow left-0 pl-16">
      <h2 className="text-neutral-300 text-xl flex-grow">Status</h2>
      <div className="pt-2">
        {isHeathy && (
          <div className="flex items-center text-green-400">
            <MdCheckCircleOutline className="mr-2" /> <span>Up to date</span>
          </div>
        )}

        {isFailling && (
          <>
            <div className="flex items-center text-red-400 mb-2">
              <MdCheckCircleOutline className="mr-2" />{' '}
              <span>Failed to apply configuration.</span>
            </div>

            <button
              onClick={() => setShowLogs(true)}
              className="text-center mb-2 text-white w-full rounded-md bg-neutral-800 hover:bg-neutral-800/50 px-2 py-1"
            >
              View logs
            </button>

            <button className="text-center text-neutral-400 hover:text-white w-full rounded-md hover:bg-neutral-800 px-2 py-1 border border-neutral-800">
              Download support bundle
            </button>

            <LogDialog
              pod={podsHasRestarts?.[0].metadata.name ?? ''}
              isOpen={showLogs}
              onClose={() => setShowLogs(false)}
            />
          </>
        )}

        {isNotReady && (
          <div className="flex items-center text-yellow-400">
            <MdWarning className="mr-2" /> <span>Not ready</span>
          </div>
        )}

        {isApplying && (
          <div className="flex items-center text-blue-400">
            <MdPending className="mr-2" /> <span>Applying</span>
          </div>
        )}
      </div>

      {!isHeathy && (
        <div className="mt-2">
          {unreadyPods?.map((p) => (
            <UnreadyPodRow key={p.metadata.name} {...p} />
          ))}
        </div>
      )}
    </div>
  )
}

export const SettingsPage: React.FC = () => {
  const [config, setConfig] = useState<any>({ bucket: {} })
  const { data, error, isInitialLoading } = useLatestConfigQuery()
  useEffect(() => {
    if (data == null) return
    setConfig(data)
  }, [config, setConfig, data])

  return (
    <>
      <TopNavbar />
      <div className="max-w-5xl mx-auto mt-10 mb-20">
        <h1 className="text-3xl font-semibold tracking-wide mb-4">Settings</h1>

        {isInitialLoading ? (
          <div>Loading...</div>
        ) : (
          <>
            {error != null ? (
              <div>
                <p>
                  Config not found! Once a config is found it will be shown
                  here.
                </p>
                <div className="text-neutral-400 mt-4">
                  <p>This can happen if</p>
                  <ul className="mt-2">
                    <li>- Weights &amp; Biases is being provisioned</li>
                    <li>
                      - You have not deployed Weights & Biases custom resource
                      definition
                    </li>
                  </ul>
                </div>
              </div>
            ) : (
              <div className="flex items-start my-12">
                <div className="grid gap-8 max-w-[700px]">
                  <LicenseCard />
                  <BucketCard />
                  <DatabaseCard />
                  <SsoCard />
                </div>
                <StatusCard />
              </div>
            )}
          </>
        )}
      </div>
    </>
  )
}
