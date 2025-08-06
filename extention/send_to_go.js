export async function sendDataToGo(id,url,filename,mime) {
        const data = {
            id: id,
            url: url,
            filename : filename,
            mime:mime
        };

        try {
            const response = await fetch('http://localhost:8080/submit-data', { 
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json'
                },
                body: JSON.stringify(data)
            });

            const result = await response.json(); // get { ok: true, status: "safe" }

        console.log("Response from Go:", result);

        if (result.ok && result.status === "safe") {
            chrome.downloads.resume(id);
        } else {
            chrome.downloads.cancel(id);
            alert("Malicious file blocked!");
        }
    } catch (err) {
        console.error("Error checking download:", err);
    }
}

