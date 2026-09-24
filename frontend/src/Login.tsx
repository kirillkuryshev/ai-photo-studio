import { useState } from 'react'
import './Login.css'
import { useNavigate } from 'react-router-dom'

type LoginProps = {
  setIsAuthenticated: React.Dispatch<React.SetStateAction<boolean>>
}

function Login({ setIsAuthenticated }: LoginProps) {
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [message, setMessage] = useState('')
  const navigate = useNavigate()

  async function handleSubmit(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault()

    const response = await fetch('http://localhost:8000/api/login', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify({
        email: email,
        password: password,
      }),
    })

    const data = await response.json()

    setMessage(data.message)

    if (response.ok) {
      localStorage.setItem('isAuthenticated', 'true')
      localStorage.setItem('token', data.token)

      setIsAuthenticated(true)
      navigate('/')
    }
  }

  return (
    <div className="login">
      <h1>Log In</h1>

      <form onSubmit={handleSubmit}>
        <input
          type="email"
          placeholder="Email"
          value={email}
          onChange={(event) => setEmail(event.target.value)}
        />

        <input
          type="password"
          placeholder="Password"
          value={password}
          onChange={(event) => setPassword(event.target.value)}
        />

        <button type="submit">
          Log In
        </button>
      </form>

      <p>{message}</p>
    </div>
  )
}

export default Login