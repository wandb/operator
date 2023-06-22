import { useMutation, useQuery } from '@tanstack/react-query'
import axios from 'axios'
import { client } from '../client'

axios.defaults.withCredentials = true

export const useViewer = () => {
  return useQuery({
    queryKey: ['viewer'],
    queryFn: async () =>
      (
        await axios.get<{ isPasswordSet: boolean; loggedIn: boolean }>(
          'http://localhost:9090/api/v1/viewer',
        )
      ).data,
  })
}

export const useSetPasswordMutation = () => {
  return useMutation(
    (d: string) => {
      return axios.post('http://localhost:9090/api/v1/password', d)
    },
    {
      onSuccess: () => {
        client.invalidateQueries(['viewer'])
      },
    },
  )
}

export const useLoginMutation = () => {
  return useMutation(
    (d: string) => {
      return axios.post('http://localhost:9090/api/v1/login', d)
    },
    {
      onSuccess: () => {
        client.invalidateQueries(['viewer'])
      },
    },
  )
}
export const useLogoutMutation = () => {
  return useMutation(
    () => {
      return axios.get('http://localhost:9090/api/v1/logout')
    },
    {
      onSuccess: () => {
        client.invalidateQueries(['viewer'])
      },
    },
  )
}

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
      return axios.post('http://localhost:9090/api/v1/config/latest', d)
    },
    {
      onSuccess: () => {
        client.invalidateQueries(['latest', 'config'])
      },
    },
  )
}
