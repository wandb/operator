import { Link, NavLink } from 'react-router-dom'
import { WBLogo } from './WBLogo'
import { MdLogout } from 'react-icons/md'
import { useLogoutMutation } from './api/auth'

const DashboardButton: React.FC = () => {
  return (
    <NavLink
      to="/"
      className={({ isActive }) =>
        `rounded-full p-1 px-3 ${
          isActive ? 'cursor-default' : 'hover:bg-neutral-800 text-neutral-400'
        }`
      }
    >
      Dashboard
    </NavLink>
  )
}

const SettingsButton: React.FC = () => {
  return (
    <NavLink
      to="/settings"
      className={({ isActive }) =>
        `rounded-full p-1 px-3 ${
          isActive ? 'cursor-default' : 'hover:bg-neutral-800 text-neutral-400'
        }`
      }
    >
      Settings
    </NavLink>
  )
}

const EditorButton: React.FC = () => {
  return (
    <NavLink
      to="/editor"
      className={({ isActive }) =>
        `rounded-full p-1 px-3 ${
          isActive ? 'cursor-default' : 'hover:bg-neutral-800 text-neutral-400'
        }`
      }
    >
      Editor
    </NavLink>
  )
}

type TopNavbarProps = {
  maxW?: string
  title?: string
}

export const TopNavbar: React.FC<TopNavbarProps> = ({
  title = 'W&B Server Console',
  maxW = '5xl',
}) => {
  const { mutate: logout } = useLogoutMutation()
  return (
    <>
      <div className="fixed top-0 z-50 w-full border-b border-neutral-200 bg-white dark:border-neutral-800 dark:bg-neutral-900">
        <div className={`py-0.25 container mx-auto max-w-${maxW}`}>
          <div className="flex items-center justify-between">
            <div className="flex items-center justify-start">
              <Link to="/" className="-ml-4 flex md:mr-10">
                <WBLogo />
                <span className="self-center whitespace-nowrap font-serif text-xl dark:text-white sm:text-xl">
                  {title}
                </span>
              </Link>
            </div>

            <div className="flex items-center gap-1">
              <DashboardButton />
              <SettingsButton />
              <EditorButton />

              <button
                className="p-2 ml-2 rounded-full hover:bg-neutral-50/5 m-2"
                onClick={() => logout()}
              >
                <MdLogout className="text-neutral-400 hover:text-neutral-500 cursor-pointer" />
              </button>
            </div>
          </div>
        </div>
      </div>

      <div className="h-14" />
    </>
  )
}
