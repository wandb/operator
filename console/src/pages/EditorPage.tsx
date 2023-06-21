import CodeMirror from '@uiw/react-codemirror'
import { json } from '@codemirror/lang-json'
import React, { useCallback, useEffect, useMemo, useState } from 'react'
import { vscodeDark } from '@uiw/codemirror-theme-vscode'
import { useConfigMutation, useConfigQuery } from '../common/api'
import { TopNavbar } from '../common/TopNavbar'

export const EditorPage: React.FC = () => {
  const [config, setConfig] = useState('')

  const format = useCallback(() => {
    setConfig((c) => JSON.stringify(JSON.parse(c), null, 2))
  }, [setConfig])

  const isValidJson = useMemo(() => {
    try {
      JSON.parse(config)
      return true
    } catch {
      return false
    }
  }, [config])

  const { isLoading, data } = useConfigQuery()
  const setCurrentConfig = useCallback(
    () => setConfig(JSON.stringify(data, null, 2)),
    [setConfig, data],
  )

  useEffect(() => setCurrentConfig(), [setCurrentConfig])

  const { mutate } = useConfigMutation()
  console.log(config)
  if (isLoading && config != '') return <div>Loading...</div>
  return (
    <>
      <TopNavbar />
      <div className="max-w-5xl mx-auto mt-10">
        <h1 className="text-3xl font-semibold tracking-wide mb-4">Editor</h1>
        <div
          className="p-2 rounded-md drop-shadow-xl"
          style={{ backgroundColor: '#1e1e1e' }}
        >
          <CodeMirror
            onKeyDown={(e) => {
              if (e.ctrlKey && e.key == 's' && isValidJson) {
                format()
                e.preventDefault()
              }
            }}
            value={config}
            theme={vscodeDark}
            extensions={[json()]}
            onChange={(v) => setConfig(v)}
          />
        </div>

        <div className="py-4">
          <button
            disabled={!isValidJson}
            onClick={() => mutate(JSON.parse(config))}
            className="bg-green-600 hover:bg-green-500 disabled:opacity-50 rounded-md text-white px-4 py-2"
          >
            Save
          </button>
        </div>
      </div>
    </>
  )
}
