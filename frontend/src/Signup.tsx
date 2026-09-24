import { useState } from 'react'
import './Signup.css'
import { useNavigate } from 'react-router-dom'

type SignupProps = {
  setIsAuthenticated: React.Dispatch<React.SetStateAction<boolean>>
}

function Signup({ setIsAuthenticated }: SignupProps) {
  const [name, setName] = useState('')
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [message, setMessage] = useState('')
  const navigate = useNavigate()

  async function handleSignup(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault()

    const response = await fetch('http://localhost:8000/api/signup', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify({
        name:name,
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
    <div className="signup">
      <h1>Sign up</h1>

      <form onSubmit={handleSignup}>

        <input
          type="name"
          placeholder="Name"
          value={name}
          onChange={(event) => setName(event.target.value)}
        />

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
          Sign up
        </button>
      </form>

      <p>{message}</p>

    </div>
  )
  
}

export default Signup