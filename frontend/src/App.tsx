import { useState } from 'react'
import './App.css'
import Login from './Login'
import Signup from './Signup'
import Header from './Header'
import { Routes, Route, Link } from 'react-router-dom'
import RestoreOldPhotos from './RestoreOldPhotos'
import UpscaleImage from './UpscaleImage'
import UserHistory from './UserHistory'

const aiTools = [
  {
    name: 'Restore Old Photos',
    description: 'Restore damaged and old photographs',
    available: true,
  },
  {
    name: 'Colorize Photos',
    description: 'Add color to black and white photos',
    available: false,
  },
  {
    name: 'Upscale Image',
    description: 'Increase image resolution to 4K',
    available: true,
  },
  {
    name: 'Enhance Quality',
    description: 'Improve photo quality with AI',
    available: false,
  },
  {
    name: 'Remove Noise',
    description: 'Remove noise from photographs',
    available: false,
  },
  {
    name: 'Remove Scratches',
    description: 'Remove scratches and damage',
    available: false,
  },
]

function Home() {
  return (
    <div className="ai-grid">
      {aiTools.map((tool) =>
        tool.available ? (
          <Link
            key={tool.name}
            to={tool.name === 'Upscale Image' ? '/upscale-4k' : '/restore-old-photos'}
            className="available-card"
          >
            <h2>{tool.name}</h2>
            <p>{tool.description}</p>
          </Link>
        ) : (
          <div
            key={tool.name}
            className="unavailable-card"
          >
            <h2>{tool.name}</h2>
            <p>{tool.description}</p>
          </div>
        )
      )}
    </div>
  )
}

function App() {
  const [isAuthenticated, setIsAuthenticated] = useState(
    localStorage.getItem('isAuthenticated') === 'true'
  )

  return (
    <>
      <Header
        isAuthenticated={isAuthenticated}
        setIsAuthenticated={setIsAuthenticated}
      />

      <Routes>
        <Route path="/" element={<Home />} />
        <Route
          path="/login"
          element={
            <Login setIsAuthenticated={setIsAuthenticated} />
          }
        />
        <Route 
          path="/signup" 
          element={
            <Signup setIsAuthenticated={setIsAuthenticated} />
          } 
        />
                <Route
          path="/restore-old-photos"
          element={
            <>
              <Home />
              <RestoreOldPhotos />
            </>
          }
        />
        <Route
          path="/upscale-4k"
          element={
            <>
              <Home />
              <UpscaleImage />
            </>
          }
        />
        <Route 
          path="/history" 
          element={
            <UserHistory />
          } 
        />

      </Routes>
    </>
  )
}

export default App