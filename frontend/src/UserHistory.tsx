import { useEffect, useState } from 'react';
import './UserHistory.css';


interface HistoryItem {
  id: number;
  ai_model: string;
  status: string;
  image_before: string;
  image_after: string | null;
  created_at: string;
}

function UserHistory() {
  const [history, setHistory] = useState<HistoryItem[]>([]);

  useEffect(() => {
    async function fetchHistory() {
      const token = localStorage.getItem('token')

      const response = await fetch(
        'http://127.0.0.1:8000/api/user-history',
        {
          headers: {
            Authorization: `Bearer ${token}`,
          },
        }
      )
      const data = await response.json();
      setHistory(data.data);
    }
    
    fetchHistory();
  }, []);

  return (
    <div className="history-page">
      <h1>Request History</h1>
      <p>Records found: {history.length}</p>

      <div className="history-list">
        {history.map((item) => (
          <div key={item.id} className="history-item">
            <h2>{item.ai_model}</h2>
            <p>Status: {item.status}</p>
            <p>Date: {new Date(item.created_at).toLocaleString()}</p>
            
            <div className="history-images">
              <div>
                <p>Before:</p>
                <img 
                  src={"http://127.0.0.1:8000/storage/" + item.image_before} 
                  alt="Before" 
                  width="200" 
                />
              </div>
              
              {item.image_after && (
                <div>
                  <p>After:</p>
                  <img 
                    src={"http://127.0.0.1:8000/storage/" + item.image_after} 
                    alt="After" 
                    width="200" 
                  />
                </div>
              )}
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}

export default UserHistory;