import { useMutation, useQuery } from '@tanstack/react-query'
import { instance } from '../instance'
import { client } from '../../../client'

export const getLatestConfig = (namespace = 'default', name = 'wandb') =>
  instance.get<{
    version: string
    directory: string
    config: Record<string, any>
  }>(`/v1/config/${namespace}/${name}/latest`)

export const useLatestConfigQuery = (namespace = 'default', name = 'wandb') => {
  return useQuery({
    queryKey: ['latest', 'config'],
    retry: 0,
    queryFn: async () => (await getLatestConfig(namespace, name)).data,
  })
}

export const useLatestConfigMutation = (
  namespace = 'default',
  name = 'wandb',
) => {
  return useMutation(
    (body: { release?: string; config: Record<string, any> }) =>
      instance.post(`/v1/config/${namespace}/${name}/latest`, body),
    { onSuccess: () => client.invalidateQueries(['latest', 'config']) },
  )
}
