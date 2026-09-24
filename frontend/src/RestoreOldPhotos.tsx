import { useState } from 'react'
import './RestoreOldPhotos.css'
import { useNavigate } from 'react-router-dom'

function RestoreOldPhotos() {
  const [selectedFile, setSelectedFile] = useState<File | null>(null)
  const navigate = useNavigate()
  const [isProcessing, setIsProcessing] = useState(false)
  const [isCompleted, setIsCompleted] = useState(false)
  const [resultPhoto, setResultPhoto] = useState<Blob | null>(null)

  function handleFileChange(event: React.ChangeEvent<HTMLInputElement>) {
    const file = event.target.files?.[0] ?? null
    setSelectedFile(file)
  }

  async function handleTransform() {
    if (!selectedFile) {
      return
    }

    setIsProcessing(true)

    const formData = new FormData()
    formData.append('photo', selectedFile)

    const token = localStorage.getItem('token')

    const response = await fetch(
      'http://127.0.0.1:8000/api/photos/transform',
      {
        method: 'POST',
        headers: {
          Authorization: `Bearer ${token}`,
        },
        body: formData,
      }
    )

    const result = await response.blob()

    setResultPhoto(result)
    setIsProcessing(false)
    setIsCompleted(true)
  }


  return (
    <div className="modal-overlay">
        <div className="modal-window">
        <button
            className="close-button"
            onClick={() => navigate('/')}
        >
            ×
        </button>

        {!isProcessing && !isCompleted && (
          <div>
            <h1>Restore Old Photos</h1>

            <p>
              Restore damaged and old photographs with AI.
            </p>

            <label className="file-button">
            Choose Photo
            <input
                type="file"
                accept="image/*"
                onChange={handleFileChange}
            />
            </label>

            {selectedFile && (
              <p>Selected: {selectedFile.name}</p>
            )}

            <button disabled={!selectedFile} onClick={handleTransform}>
              Transform
            </button>
        
          </div>
        )}

        {isProcessing && (
          <p>Processing...</p>
        )}

        {isCompleted && resultPhoto && (
          <div>
            <p>Restored photo</p>
            <br />
            <img
              src={URL.createObjectURL(resultPhoto)}
              alt="Restored photo"
              width="400"
            />
          </div>
        )}
      </div>
    </div>
  )
}

export default RestoreOldPhotos