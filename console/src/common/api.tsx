import { useMutation, useQuery } from '@tanstack/react-query'
import axios from 'axios'
import { client } from '../client'

export const usePodListQuery = () => {
  return useQuery({
    queryKey: ['pods'],
    queryFn: async () =>
      (await axios.get('http://localhost:9090/api/v1/k8s/pods')).data,
  })
}

export const useDeploymentsQuery = () => {
  return useQuery({
    refetchInterval: 1000,
    queryKey: ['deployments'],
    queryFn: async () =>
      (await axios.get('http://localhost:9090/api/v1/k8s/deployments')).data,
  })
}

export const useServicesQuery = () => {
  return useQuery({
    queryKey: ['services'],
    queryFn: async () =>
      (await axios.get('http://localhost:9090/api/v1/k8s/services')).data,
  })
}
export const useNodesQuery = () => {
  return useQuery({
    queryKey: ['nodes'],
    refetchInterval: 1000,
    queryFn: async () =>
      (await axios.get('http://localhost:9090/api/v1/k8s/nodes')).data,
  })
}

export const useStatefulSetsQuery = () => {
  return useQuery({
    queryKey: ['stateful-sets'],
    queryFn: async () =>
      (await axios.get('http://localhost:9090/api/v1/k8s/stateful-sets')).data,
  })
}

export const useConfigQuery = () => {
  return useQuery({
    queryKey: ['latest', 'config'],
    queryFn: async () =>
      (await axios.get('http://localhost:9090/api/v1/config/latest')).data,
  })
}

export const useConfigMutation = () => {
  return useMutation(
    (d: any) => {
      return axios.post(
        'http://localhost:9090/api/v1/config/latest',
        JSON.stringify(d),
      )
    },
    {
      onSuccess: () => {
        client.invalidateQueries(['latest', 'config'])
      },
    },
  )
}
