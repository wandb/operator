import { useQuery } from '@tanstack/react-query'
import { instance } from '../instance'

export const getNodes = () =>
  instance.get<{
    items: Array<{
      name: string
      status: any
      cpu: { total: number; used: number }
      memory: { total: number; used: number }
    }>
  }>('/v1/k8s/nodes')

export const useNodesQuery = () => {
  return useQuery({
    queryKey: ['nodes'],
    refetchInterval: 10_000,
    queryFn: async () => (await getNodes()).data,
  })
}

export const getPods = (namespace = 'default') =>
  instance.get<{
    items: Array<{
      metadata: { name: string; generatedName: string }
      status: {
        phase: string
        containerStatuses: Array<{ ready: boolean }>
      }
      spec: {
        containers: Array<{
          name: string
          image: string
          nodeName: string
        }>
      }
    }>
  }>(`/v1/k8s/${namespace}/pods`)

export const usePodsQuery = (namespace = 'default') => {
  return useQuery({
    queryKey: ['pods'],
    refetchInterval: 10_000,
    queryFn: async () => (await getPods(namespace)).data,
  })
}

export const getPodLogs = (name: string, namespace = 'default') =>
  instance.get<string>(`/v1/k8s/${namespace}/pods/${name}/logs`)

export const usePodsLogsQuery = (name: string, namespace = 'default') => {
  return useQuery({
    queryKey: ['pods', name, 'logs'],
    refetchInterval: 60_000,
    queryFn: async () => (await getPodLogs(name, namespace)).data,
  })
}

export const getDeployments = (namespace = 'default') =>
  instance.get<{
    items: Array<{
      metadata: { name: string }
      status: { replicas?: number; readyReplicas?: number }
      spec: {
        template: { spec: { containers: Array<{ image: string }> } }
      }
    }>
  }>(`/v1/k8s/${namespace}/deployments`)

export const useDeploymentsQuery = (namespace = 'default') => {
  return useQuery({
    refetchInterval: 10_000,
    queryKey: ['deployments'],
    queryFn: async () => (await getDeployments(namespace)).data,
  })
}

export const getStatefulSets = (namespace = 'default') =>
  instance.get<{
    items: Array<{
      metadata: { name: string }
      status: { replicas?: number; readyReplicas?: number }
      spec: {
        template: { spec: { containers: Array<{ image: string }> } }
      }
    }>
  }>(`/v1/k8s/${namespace}/stateful-sets`)

export const useStatefulSetsQuery = (namespace = 'default') => {
  return useQuery({
    refetchInterval: 10_000,
    queryKey: ['stateful-sets'],
    queryFn: async () => (await getStatefulSets(namespace)).data,
  })
}