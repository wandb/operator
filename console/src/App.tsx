import CodeMirror from '@uiw/react-codemirror'
import { json } from '@codemirror/lang-json'
import { useMemo, useState } from 'react'
import { useMutation, useQuery } from '@tanstack/react-query'
import axios from 'axios'
import { client } from './client'
import { lintGutter } from '@codemirror/lint'
import Ajv from 'ajv'

function App() {
  const [config, setConfig] = useState<string>('null')
  const { data: schema } = useQuery({
    queryKey: ['schema'],
    queryFn: async () => {
      return (
        await axios.get(
          'https://raw.githubusercontent.com/wandb/cdk8s/main/config-schema.json',
        )
      )?.data
    },
  })
  const isValidJson = useMemo(() => {
    try {
      JSON.parse(config)
      return true
    } catch {
      return false
    }
  }, [config])

  const validResults = useMemo(() => {
    if (schema == null) return null
    const ajv = new Ajv({ allErrors: true })
    const validate = ajv.compile(schema.definitions['wandb-config-cdk'])
    if (isValidJson) {
      validate(JSON.parse(config))
      return validate.errors
    }
    return null
  }, [schema, config, isValidJson])

  const { isLoading } = useQuery({
    queryKey: ['latest', 'config'],
    queryFn: async () => {
      const req = await axios.get('http://localhost:9090/api/v1/config/latest')
      setConfig(JSON.stringify(req.data, null, 2))
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
          extensions={[json(), lintGutter()]}
          onChange={(v) => setConfig(v)}
        />
      </div>

      <div className="p-4">
        {!isValidJson && <p className="text-red-600">Invalid JSON.</p>}
        {validResults?.map((e, idx) => (
          <div key={idx} className="text-red-600">
            <p>
              <span className="text-bold font-bold">
                {e.instancePath.replaceAll('/', '.')}
              </span>
              {' -> '}
              {e.message}
            </p>
          </div>
        ))}
      </div>
      <div className="p-4">
        <button
          disabled={!isValidJson || (validResults?.length ?? 0) > 0}
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
