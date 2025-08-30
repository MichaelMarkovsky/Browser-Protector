// send_to_go.js
// returns { isSafe, proxyUrl? }
export async function sendDataToGo(id, url, filename, mime) {
  const data = {
    id: id,
    url: url,
    filename: filename,
    mime: mime
  };

  try {
    const response = await fetch('http://localhost:8080/submit-data', { 
      method: 'POST',
      headers: {
        'Content-Type': 'application/json'
      },
      body: JSON.stringify(data)
    });

    if (!response.ok) {
      throw new Error(`HTTP error! Status: ${response.status}`);
    }

    const result = await response.json(); // {isSafe: true, proxyUrl?: "http://localhost:8080/safe/<token>"}
    console.log("Response from Go:", result);
    return result; // Return result for service worker
  } catch (err) {
    console.error("Error checking download:", err);
    throw err; // Propagate error
  }
}
