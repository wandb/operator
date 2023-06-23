import React, { Fragment } from 'react'
import { Dialog, Transition } from '@headlessui/react'
import { usePodsLogsQuery } from './api/k8s'
import CodeMirror from '@uiw/react-codemirror'
import { vscodeDark } from '@uiw/codemirror-theme-vscode'

const TextLogViewer: React.FC<{ pod: string }> = ({ pod }) => {
  const logs = usePodsLogsQuery(pod)
  return (
    <div
      className="p-2 rounded-md drop-shadow-xl mt-4"
      style={{ backgroundColor: '#1e1e1e' }}
    >
      <CodeMirror value={logs.data} theme={vscodeDark} readOnly />
    </div>
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
                <Dialog.Title as="h3" className="text-lg font-medium leading-6">
                  {pod}
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
