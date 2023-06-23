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

export const usePodsQuery = (namespace = 'default') => {
  return useQuery({
    queryKey: ['pods'],
    refetchInterval: 10_000,
    queryFn: async () =>
      (
        await instance.get<{
          items: Array<{
            metadata: { name: string; generatedName: string }
            status: { phase: string }
            spec: {
              containers: Array<{
                name: string
                image: string
                nodeName: string
              }>
            }
          }>
        }>(`/v1/k8s/${namespace}/pods`)
      ).data,
  })
}

export const usePodsLogsQuery = (name: string, namespace = 'default') => {
  return useQuery({
    queryKey: ['pods', name, 'logs'],
    refetchInterval: 60_000,
    queryFn: async () =>
      (await instance.get<string>(`/v1/k8s/${namespace}/pods/${name}/logs`))
        .data,
  })
}

export const useDeploymentsQuery = (namespace = 'default') => {
  return useQuery({
    refetchInterval: 10_000,
    queryKey: ['deployments'],
    queryFn: async () =>
      (
        await instance.get<{
          items: Array<{
            metadata: { name: string }
            status: { replicas?: number; readyReplicas?: number }
            spec: {
              template: { spec: { containers: Array<{ image: string }> } }
            }
          }>
        }>(`/v1/k8s/${namespace}/deployments`)
      ).data,
  })
}
export const useStatefulSetsQuery = (namespace = 'default') => {
  return useQuery({
    refetchInterval: 10_000,
    queryKey: ['stateful-sets'],
    queryFn: async () =>
      (
        await instance.get<{
          items: Array<{
            metadata: { name: string }
            status: { replicas?: number; readyReplicas?: number }
            spec: {
              template: { spec: { containers: Array<{ image: string }> } }
            }
          }>
        }>(`/v1/k8s/${namespace}/stateful-sets`)
      ).data,
  })
}
