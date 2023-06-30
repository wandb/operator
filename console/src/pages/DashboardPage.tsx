import React, { useEffect, useState } from 'react'
import { TopNavbar } from '../common/TopNavbar'
import { MdArrowDropUp, MdCircle } from 'react-icons/md'
import { Area, AreaChart, ResponsiveContainer, XAxis, YAxis } from 'recharts'
import colors from 'tailwindcss/colors'
import { instance } from '../common/api/instance'
import { sum } from 'lodash'
import { format } from 'date-fns'
import {
  getDeployments,
  getEvents,
  getNamespaces,
  getNodes,
  getPodLogs,
  getPods,
  getServices,
  getStatefulSets,
  useDeploymentsQuery,
  useNodesQuery,
  usePodsQuery,
  useStatefulSetsQuery,
} from '../common/api/k8s'
import {
  getAppliedConfig,
  getLatestConfig,
  getWandbSpec,
  useAppliedConfigQuery,
} from '../common/api/config'
import { LogDialog } from '../common/PodLogs'
import JSZip from 'jszip'
import { saveAs } from 'file-saver'

const PodRow: React.FC<{
  metadata: { name: string }
  status: { phase: string; containerStatuses: Array<{ ready: boolean }> }
}> = (item) => {
  const [showLogs, setShowLogs] = useState(false)
  const ready = item.status.containerStatuses?.every((c) => c.ready)
  return (
    <>
      <button
        className="flex items-center gap-2 hover:bg-neutral-700/50 w-full rounded-md px-2"
        onClick={() => setShowLogs(true)}
      >
        <div className="mr-2">
          {ready ? (
            <MdCircle className="text-green-400 w-3" />
          ) : (
            <MdCircle className="text-red-400 w-3" />
          )}
        </div>
        <div>
          {item.metadata.name.replace('wandb-', '')}{' '}
          <span className="text-neutral-500">
            ({ready ? item.status.phase : 'Unhealthy'})
          </span>
        </div>
      </button>
      <LogDialog
        pod={item.metadata.name}
        isOpen={showLogs}
        onClose={() => setShowLogs(false)}
      />
    </>
  )
}

const PodsStatus: React.FC = () => {
  const deployments = usePodsQuery()

  const items = deployments.data?.items ?? []

  if (items.length === 0 && !deployments.isLoading) return null

  return (
    <div className="rounded-md bg-neutral-800 px-4 py-3">
      <div className="text-lg mb-1">Pods</div>
      {items.map((item) => (
        <PodRow key={item.metadata.name} {...item} />
      ))}
    </div>
  )
}

const DeploymentsStatus: React.FC = () => {
  const deployments = useDeploymentsQuery()
  const statefulSets = useStatefulSetsQuery()
  const items = [
    ...(deployments.data?.items ?? []),
    ...(statefulSets.data?.items ?? []),
  ].sort((a: any, b: any) => a.metadata.name.localeCompare(b.metadata.name))

  if (items.length === 0 && !deployments.isLoading) return null
  return (
    <div className="rounded-md bg-neutral-800 px-4 py-3">
      <div className="text-lg mb-1">Deployments</div>
      {items.map((item) => {
        const { readyReplicas, replicas } = item.status
        const images = item.spec.template.spec.containers.map((c) => c.image)
        return (
          <div key={item.metadata.name} className="flex items-center gap-2">
            <div>
              {readyReplicas === replicas ? (
                <MdCircle className="text-green-400 w-3" />
              ) : (
                <MdCircle className="text-red-400 w-3" />
              )}
            </div>
            <div className="w-7 text-neutral-400 text-sm">
              {readyReplicas ?? 0}/{replicas}
            </div>
            <div>
              {item.metadata.name.replace('wandb-', '')}{' '}
              <span className="text-neutral-500">({images.join(', ')})</span>
            </div>
          </div>
        )
      })}
    </div>
  )
}

const NodesStatus: React.FC = () => {
  const nodes = useNodesQuery()
  const items = (nodes.data?.items ?? []).sort((a: any, b: any) =>
    a.name.localeCompare(b.name),
  )
  return (
    <div className="rounded-md bg-neutral-800 px-4 py-3">
      <div className="text-lg mb-1">Nodes</div>
      <div className="grid gap-1">
        {items.map((item: any) => {
          const name = item.name
          const { capacity } = item.status

          return (
            <div key={name} className="flex">
              <div className="mt-1.5 mr-3 text-green-400">
                <MdCircle className="w-3" />
              </div>

              <div>
                <div>{name}</div>
                <div className="text-neutral-400 text-sm">
                  {capacity.cpu ?? 0} cpu / {capacity.memory ?? 0} memory
                </div>
              </div>
            </div>
          )
        })}
      </div>
    </div>
  )
}

const CpuUsage: React.FC = () => {
  const [data, setData] = useState<Array<{ date: Date; value: number }>>([])
  useEffect(() => {
    const getCpu = async () => {
      const nodes = await getNodes()
      const date = new Date()
      const used = sum(nodes.data.items.map((d: any) => d.cpu.used))
      const total = sum(nodes.data.items.map((d: any) => d.cpu.total))

      setData((d) => {
        const value = (used / total) * 100
        return [...d, { date, value }].slice(-120)
      })
    }

    getCpu()
    const interval = setInterval(getCpu, 5_000)

    return () => {
      clearInterval(interval)
    }
  }, [])

  return (
    <div className="rounded-md bg-neutral-800 px-4 py-3">
      <div className="text-lg mb-1">CPU Usage</div>
      <div className="h-[200px] text-xs">
        <ResponsiveContainer>
          <AreaChart
            data={data}
            margin={{
              right: 10,
              top: 4,
              bottom: -5,
              left: -20,
            }}
          >
            <defs>
              <linearGradient id="cpu" x1="0" y1="0" x2="0" y2="1">
                <stop
                  offset="5%"
                  stopColor={colors.blue[500]}
                  stopOpacity={0.4}
                />
                <stop
                  offset="95%"
                  stopColor={colors.blue[500]}
                  stopOpacity={0}
                />
              </linearGradient>
            </defs>
            <XAxis
              stroke={colors.neutral[500]}
              dataKey="date"
              tickFormatter={(d) => format(new Date(d), 'mm:ss')}
            />
            <YAxis
              type="number"
              dataKey="value"
              stroke={colors.neutral[500]}
              tickFormatter={(d: number) => `${d.toFixed(0)}%`}
            />
            <Area
              dataKey="value"
              type="monotone"
              stroke={colors.blue[400]}
              strokeWidth={2}
              fillOpacity={1}
              fill="url(#cpu)"
            />
          </AreaChart>
        </ResponsiveContainer>
      </div>
    </div>
  )
}

const MemoryUsage: React.FC = () => {
  const [data, setData] = useState<Array<{ date: Date; value: number }>>([])
  useEffect(() => {
    const getMemory = async () => {
      const nodes = await instance.get('/v1/k8s/nodes')
      const now = new Date()
      const used = sum(nodes.data.items.map((d: any) => d.memory.used))
      const total = sum(nodes.data.items.map((d: any) => d.memory.total))
      setData((d) => {
        const value = (used / total) * 100
        return [...d, { date: now, value }].slice(-120)
      })
    }
    getMemory()
    const interval = setInterval(getMemory, 5_000)

    return () => {
      clearInterval(interval)
    }
  }, [])

  return (
    <div className="rounded-md bg-neutral-800 px-4 py-3">
      <div className="text-lg mb-1">Memory Usage</div>
      <div className="h-[200px] text-xs">
        <ResponsiveContainer>
          <AreaChart
            data={data}
            margin={{
              right: 10,
              top: 4,
              bottom: -5,
              left: -20,
            }}
          >
            <defs>
              <linearGradient id="mem" x1="0" y1="0" x2="0" y2="1">
                <stop
                  offset="5%"
                  stopColor={colors.green[500]}
                  stopOpacity={0.4}
                />
                <stop
                  offset="95%"
                  stopColor={colors.green[500]}
                  stopOpacity={0}
                />
              </linearGradient>
            </defs>
            <XAxis
              stroke={colors.neutral[500]}
              dataKey="date"
              tickFormatter={(d) => format(new Date(d), 'mm:ss')}
            />
            <YAxis
              type="number"
              dataKey="value"
              stroke={colors.neutral[500]}
              tickFormatter={(d: number) => `${d.toFixed(0)}%`}
            />
            <Area
              dataKey="value"
              type="monotone"
              stroke={colors.green[400]}
              strokeWidth={2}
              fillOpacity={1}
              fill="url(#mem)"
            />
          </AreaChart>
        </ResponsiveContainer>
      </div>
    </div>
  )
}

const SystemInfo: React.FC = () => {
  const config = useAppliedConfigQuery()
  return (
    <div className="rounded-md bg-neutral-800 px-4 py-3">
      <div className="text-lg mb-1">System Info</div>
      <div className="flex items-center">
        {!config.isError ? (
          <>
            <div className="font-semibold text-neutral-400 mr-2">Version</div>
            <div>{config.data?.version}</div>
          </>
        ) : (
          <>
            <div className="text-neutral-400">
              <p>Could not find version information.</p>
            </div>
          </>
        )}
      </div>
    </div>
  )
}

async function promiseAllWithNullOnFail<T>(
  promises: Promise<T>[],
): Promise<(T | null)[]> {
  return Promise.all(
    promises.map((promise) =>
      promise.catch((err) => {
        console.error(err)
        return null
      }),
    ),
  )
}

const Debug: React.FC = () => {
  const [loading, setLoading] = useState(false)
  const download = async () => {
    const zip = new JSZip()
    setLoading(true)

    try {
      // Get k8s resources
      const [
        nodes,
        pods,
        deployments,
        statefulSets,
        events,
        namespaces,
        services,
      ] = await promiseAllWithNullOnFail([
        getNodes(),
        getPods(),
        getDeployments(),
        getStatefulSets(),
        getEvents(),
        getNamespaces(),
        getServices(),
      ])
      const k8s = zip.folder('k8s')
      k8s?.file('nodes.json', JSON.stringify(nodes, null, 2))
      k8s?.file('pods.json', JSON.stringify(pods, null, 2))
      k8s?.file('deployments.json', JSON.stringify(deployments, null, 2))
      k8s?.file('stateful-sets.json', JSON.stringify(statefulSets, null, 2))
      k8s?.file('events.json', JSON.stringify(events, null, 2))
      k8s?.file('namespaces.json', JSON.stringify(namespaces, null, 2))
      k8s?.file('services.json', JSON.stringify(services, null, 2))

      // Get pod logs
      const logs = zip.folder('logs')
      const podLogs = await promiseAllWithNullOnFail(
        pods?.data.items.map(async (p) => ({
          name: p.metadata.name,
          logs: await getPodLogs(p.metadata.name),
        })) ?? [],
      )
      for (const p of podLogs) {
        if (p == null) continue
        logs?.file(`${p.name}.log`, p.logs.data)
      }

      // Get config
      const [latestConfig, appliedConfig, wandbSpec] =
        await promiseAllWithNullOnFail([
          getLatestConfig(),
          getAppliedConfig(),
          getWandbSpec(),
        ])

      const config = zip.folder('config')
      config?.file('latest.json', JSON.stringify(latestConfig?.data, null, 2))
      config?.file('applied.json', JSON.stringify(appliedConfig?.data, null, 2))
      config?.file('spec.json', JSON.stringify(wandbSpec?.data, null, 2))

      // Download bundle
      const blob = await zip.generateAsync({ type: 'blob' })
      const d = new Date().toISOString()
      saveAs(blob, `bundle-${d}.zip`)
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="rounded-md bg-neutral-800 px-4 py-3">
      <div className="text-lg mb-2">Support Bundle</div>
      {/* <div className="grid mb-4 mt-2 grid-cols-3">
        <div className="flex items-center ">
          <input
            id="default-checkbox"
            type="checkbox"
            checked
            className="w-4 h-4 text-blue-600 bg-neutral-100 border-neutral-300 rounded focus:ring-blue-500 dark:focus:ring-blue-600 dark:ring-offset-neutral-800 focus:ring-2 dark:bg-neutral-700 dark:border-neutral-600"
          />
          <label
            htmlFor="default-checkbox"
            className="ml-2 text-sm  text-neutral-900 dark:text-neutral-300"
          >
            Pod logs
          </label>
        </div>
      </div> */}
      <button
        disabled={loading}
        onClick={() => download()}
        className="rounded-md text-center w-full bg-neutral-700 p-2 hover:bg-neutral-500/50 disabled:opacity-50"
      >
        Download Bundle
      </button>
    </div>
  )
}

export const DashboardPage: React.FC = () => {
  const [showMore, setShowMore] = useState(false)
  return (
    <>
      <TopNavbar />
      <div className="max-w-5xl mx-auto mt-10">
        <h1 className="text-3xl font-semibold tracking-wide mb-4">Dashboard</h1>
        <div className="grid grid-cols-2 gap-4">
          <SystemInfo />
          <Debug />
          <PodsStatus />
          <DeploymentsStatus />

          <CpuUsage />
          <MemoryUsage />
        </div>
        <div className="mt-4">
          <button
            onClick={() => setShowMore(!showMore)}
            className={`w-full text-left flex items-center px-2 my-4 hover:bg-neutral-800 rounded-md`}
          >
            <MdArrowDropUp className={`mr-2 ${!showMore && 'rotate-90'}`} />
            {showMore ? 'Hide' : 'More'}
          </button>
          {showMore && (
            <div className="grid grid-cols-2 gap-4 rounded-lg border border-neutral-800 p-4">
              <NodesStatus />
            </div>
          )}
        </div>
      </div>
    </>
  )
}
