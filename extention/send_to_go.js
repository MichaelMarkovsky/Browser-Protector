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

            const result = await response.text();
            console.log("Go server response:", result); // should log: Data received successfully!
        } catch (error) {
            console.error('Error:', error);
        }
    }

