import CodeMirror from '@uiw/react-codemirror'
import { json } from '@codemirror/lang-json'
import { useState } from 'react'
import { useMutation, useQuery } from '@tanstack/react-query'
import axios from 'axios'
import { client } from './client'

function App() {
  const [config, setConfig] = useState<string>('')
  const { isLoading } = useQuery({
    queryKey: ['latest', 'config'],
    queryFn: async () => {
      const req = await axios.get('http://localhost:9090/api/v1/config/latest')
      setConfig(JSON.stringify(req.data, null, 4))
      return req.data
    },
  })

  const { mutate } = useMutation(
    (d: string) => {
      return axios.post('http://localhost:9090/api/v1/config/latest', d)
    },
    {
      onSuccess: () => {
        client.invalidateQueries(['latest', 'config'])
      },
    },
  )

  if (isLoading && config != '') return <div>Loading...</div>

  return (
    <>
      <div>
        <CodeMirror
          value={config}
          extensions={[json()]}
          onChange={(v) => setConfig(v)}
        />
      </div>

      <div className="p-4">
        <button
          onClick={() => mutate(config)}
          className="bg-green-600 hover:bg-green-500 disabled:opacity-50 rounded-md text-white px-4 py-2"
        >
          Save
        </button>
      </div>
    </>
  )
}

export default App
