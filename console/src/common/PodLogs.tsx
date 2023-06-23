import React, { Fragment, useMemo, useState } from 'react'
import { Dialog, Transition } from '@headlessui/react'
import { usePodsLogsQuery } from './api/k8s'
import { MdOutlineDownload, MdOutlineWarning, MdSort } from 'react-icons/md'

const GorillaLine: React.FC<{ children: Record<string, any> }> = ({
  children,
}) => {
  const isInfo = children.level === 'INFO'
  const [time] = children.time.split('.')
  return (
    <>
      <span className={`${isInfo && 'text-blue-500'}`}>{children.level}</span>{' '}
      <span className={`text-neutral-500`}>{time}</span>{' '}
      <span className="message">{children.message}</span>
    </>
  )
}

const LogLine: React.FC<{ children: string }> = ({ children }) => {
  const line = useMemo(() => {
    try {
      return JSON.parse(children)
    } catch {
      return children
    }
  }, [children])

  const isWarning = children.toLowerCase().includes('warn')
  const isError = children.toLowerCase().includes('error')
  const isPanic = children.toLowerCase().includes('panic')
  return (
    <div
      className={`${(isError || isPanic) && 'bg-red-900 text-white'} ${
        isWarning && 'bg-yellow-900 text-white'
      }
      text-neutral-300
      font-mono
      `}
    >
      {typeof line === 'string' ? line : <GorillaLine>{line}</GorillaLine>}
    </div>
  )
}

function downloadString(exportStr: string, exportName: string) {
  const dataStr =
    'data:text/json;charset=utf-8,' + encodeURIComponent(exportStr)
  const downloadAnchorNode = document.createElement('a')
  downloadAnchorNode.setAttribute('href', dataStr)
  downloadAnchorNode.setAttribute('download', exportName)
  document.body.appendChild(downloadAnchorNode) // required for firefox
  downloadAnchorNode.click()
  downloadAnchorNode.remove()
}

const TextLogViewer: React.FC<{ pod: string }> = ({ pod }) => {
  const [showOnlyAlerts, setShowOnlyAlerts] = useState(false)
  const [reverse, setReverse] = useState(false)
  const logs = usePodsLogsQuery(pod)
  let lines = logs.data?.split('\n')
  if (showOnlyAlerts) {
    lines = lines?.filter(
      (l) =>
        l.toLowerCase().includes('error') ||
        l.toLowerCase().includes('panic') ||
        l.toLowerCase().includes('warn'),
    )
  }
  if (reverse) {
    lines = lines?.reverse()
  }
  return (
    <>
      <div className="my-4 flex items-center gap-3">
        <button
          className="bg-blue-600 rounded-md px-3 py-1 flex items-center"
          onClick={() => downloadString(logs.data ?? '', `${pod}.logs`)}
        >
          <MdOutlineDownload className="inline-block mr-2 mt-0.5" />
          Download
        </button>
        <button
          className="bg-red-600 rounded-md px-3 py-1 flex items-center"
          onClick={() => setShowOnlyAlerts(!showOnlyAlerts)}
        >
          <MdOutlineWarning className="inline-block mr-2 mt-0.5" />
          {showOnlyAlerts ? 'Show all logs' : 'Show only alerts'}
        </button>
        <button
          className="bg-neutral-600 rounded-md px-3 py-1 flex items-center"
          onClick={() => setReverse(!reverse)}
        >
          <MdSort className="inline-block mr-2 mt-0.5" />
          {reverse ? 'Oldest → Newest' : 'Newest → Oldest'}
        </button>
      </div>
      <div
        className="p-2 rounded-md drop-shadow-xl mt-4 overflow-x-auto text-sm"
        style={{ backgroundColor: '#1e1e1e' }}
      >
        {/* <CodeMirror value={logs.data} theme={vscodeDark} readOnly /> */}
        {lines?.map((l, idx) => (
          <LogLine key={idx}>{l}</LogLine>
        ))}
      </div>
    </>
  )
}

export const LogDialog: React.FC<{
  pod: string
  isOpen: boolean
  onClose: () => void
}> = ({ pod, isOpen, onClose }) => {
  return (
    <Transition appear show={isOpen} as={Fragment}>
      <Dialog as="div" className="relative z-50" onClose={onClose}>
        <Transition.Child
          as={Fragment}
          enter="ease-out duration-300"
          enterFrom="opacity-0"
          enterTo="opacity-100"
          leave="ease-in duration-200"
          leaveFrom="opacity-100"
          leaveTo="opacity-0"
        >
          <div className="fixed inset-0 bg-black bg-opacity-25" />
        </Transition.Child>

        <div className="fixed inset-0 overflow-y-auto">
          <div className="flex items-center justify-center p-4 text-center">
            <Transition.Child
              as={Fragment}
              enter="ease-out duration-300"
              enterFrom="opacity-0 scale-95"
              enterTo="opacity-100 scale-100"
              leave="ease-in duration-200"
              leaveFrom="opacity-100 scale-100"
              leaveTo="opacity-0 scale-95"
            >
              <Dialog.Panel className="w-full transform overflow-hidden rounded-2xl bg-neutral-700 p-6 text-left align-middle shadow-xl transition-all">
                <Dialog.Title
                  as="h3"
                  className="flex items-center text-lg font-medium leading-6"
                >
                  <div className="flex-grow">{pod}</div>
                </Dialog.Title>

                {isOpen && <TextLogViewer pod={pod} />}
              </Dialog.Panel>
            </Transition.Child>
          </div>
        </div>
      </Dialog>
    </Transition>
  )
}
