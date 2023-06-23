import React, { useEffect, useState } from 'react'
import { TopNavbar } from '../common/TopNavbar'
import { MdCircle } from 'react-icons/md'
import { Area, AreaChart, ResponsiveContainer, XAxis, YAxis } from 'recharts'
import colors from 'tailwindcss/colors'
import { instance } from '../common/api/instance'
import { sum } from 'lodash'
import { format } from 'date-fns'
import {
  getNodes,
  useDeploymentsQuery,
  useNodesQuery,
  usePodsQuery,
  useStatefulSetsQuery,
} from '../common/api/k8s'
import { useLatestConfigQuery } from '../common/api/config'
import { LogDialog } from '../common/PodLogs'

const PodRow: React.FC<{
  metadata: { name: string }
  status: { phase: string }
}> = (item) => {
  const [showLogs, setShowLogs] = useState(false)
  return (
    <>
      <button
        className="flex items-center gap-2"
        onClick={() => setShowLogs(true)}
      >
        <div className="mr-2">
          {item.status.phase === 'Running' ? (
            <MdCircle className="text-green-400 w-3" />
          ) : (
            <MdCircle className="text-red-400 w-3" />
          )}
        </div>
        <div>
          {item.metadata.name.replace('wandb-', '')}{' '}
          <span className="text-neutral-500">({item.status.phase})</span>
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

  return (
    <div className="rounded-md bg-neutral-800 px-4 py-3">
      <div className="text-lg mb-1">Instances</div>
      {items.map((item) => (
        <PodRow key={item.metadata.name} {...item} />
      ))}
    </div>
  )
}

const ServicesStatus: React.FC = () => {
  const deployments = useDeploymentsQuery()
  const statefulSets = useStatefulSetsQuery()
  const items = [
    ...(deployments.data?.items ?? []),
    ...(statefulSets.data?.items ?? []),
  ].sort((a: any, b: any) => a.metadata.name.localeCompare(b.metadata.name))

  return (
    <div className="rounded-md bg-neutral-800 px-4 py-3">
      <div className="text-lg mb-1">Services</div>
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

const Version: React.FC = () => {
  const config = useLatestConfigQuery()
  return (
    <div className="rounded-md bg-neutral-800 px-4 py-3 col-span-2">
      <div className="text-lg mb-1">System Info</div>
      <div className="flex items-center">
        <div className="font-semibold text-neutral-400 mr-2">Version</div>
        <div>{config.data?.version}</div>
      </div>
    </div>
  )
}

export const DashboardPage: React.FC = () => {
  return (
    <>
      <TopNavbar />
      <div className="max-w-5xl mx-auto mt-10">
        <h1 className="text-3xl font-semibold tracking-wide mb-4">Dashboard</h1>
        <div className="grid grid-cols-2 gap-4">
          <Version />
          <PodsStatus />
          <ServicesStatus />
          <NodesStatus />
          <CpuUsage />
          <MemoryUsage />
        </div>
      </div>
    </>
  )
}
