import { Link } from 'react-router-dom'
import './Header.css'

type HeaderProps = {
  isAuthenticated: boolean
  setIsAuthenticated: React.Dispatch<React.SetStateAction<boolean>>
}

function Header({ isAuthenticated, setIsAuthenticated }: HeaderProps) {
  async function handleLogout() {
    const token = localStorage.getItem('token')

    await fetch('http://127.0.0.1:8000/api/logout', {
      method: 'POST',
      headers: {
        Authorization: `Bearer ${token}`,
      },
    })

    localStorage.removeItem('token')
    localStorage.removeItem('isAuthenticated')

    setIsAuthenticated(false)
  }

  return (
    <header className="header">
      <Link to="/">Photo Studio AI</Link>

      <div>
        <p>Enhance photos with AI</p>
      </div>

      <div className="profile-menu">
        {isAuthenticated ? (
          <>
            <button>Profile</button>

            <div className="dropdown-menu">
              <Link to="/history" className="dropdown-item">Request History</Link>
              <button onClick={handleLogout} className="dropdown-item">
                Logout
              </button>
            </div>
          </>
        ) : (
          <>
            <Link to="/login">Sign In</Link>
            <Link to="/signup">Sign Up</Link>
          </>
        )}
      </div>
    </header>
  )
}

export default Header