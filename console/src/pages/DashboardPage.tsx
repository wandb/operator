import React from 'react'
import { TopNavbar } from '../common/TopNavbar'
import { useDeploymentsQuery, useStatefulSetsQuery } from '../common/api'
import { MdCircle } from 'react-icons/md'

const ServicesStatus: React.FC = () => {
  const deployments = useDeploymentsQuery()
  const statefulSets = useStatefulSetsQuery()
  return (
    <div className="rounded-md bg-neutral-800 px-4 py-3">
      <div className="text-lg mb-1">Services</div>
      {deployments.data?.items.map((item: any) => {
        const { readyReplicas, replicas } = item.status
        const images = item.spec.template.spec.containers.map(
          (c: any) => c.image,
        )
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

      {statefulSets.data?.items.map((item: any) => {
        const { readyReplicas, replicas } = item.status
        const images = item.spec.template.spec.containers.map(
          (c: any) => c.image,
        )
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

export const DashboardPage: React.FC = () => {
  return (
    <>
      <TopNavbar />
      <div className="max-w-5xl mx-auto mt-10">
        <h1 className="text-3xl font-semibold tracking-wide mb-4">Dashboard</h1>
        <div className="grid grid-cols-2 gap-4">
          <ServicesStatus />
        </div>
      </div>
    </>
  )
}
