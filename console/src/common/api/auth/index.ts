import { useMutation, useQuery } from '@tanstack/react-query'
import { instance } from '../instance'
import { client } from '../../../client'

export const useProfileQuery = () => {
  return useQuery({
    queryKey: ['profile'],
    queryFn: async () =>
      (
        await instance.get<{ isPasswordSet: boolean; isLoggedIn: boolean }>(
          '/v1/auth/profile',
        )
      ).data,
  })
}

export const useLogoutMutation = () => {
  return useMutation(() => instance.post('/v1/auth/logout'), {
    onSuccess: () => client.invalidateQueries(['profile']),
  })
}

export const useLoginMutation = () => {
  return useMutation(
    (password: string) => instance.post('/v1/auth/login', { password }),
    { onSuccess: () => client.invalidateQueries(['profile']) },
  )
}

export const usePasswordMutation = () => {
  return useMutation(
    (password: string) => instance.post('/v1/auth/password', { password }),
    { onSuccess: () => client.invalidateQueries(['profile']) },
  )
}
