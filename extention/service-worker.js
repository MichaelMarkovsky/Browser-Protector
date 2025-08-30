// service-worker.js
import { sendDataToGo } from './send_to_go.js';

// Track processed downloads to avoid duplicates
const processedDownloads = new Set();
const safeDownloads = new Set(); // Track safe download URLs to prevent re-interception

// Map common MIME types to extensions
const mimeToExtension = {
  'application/pdf': '.pdf',
  'image/jpeg': '.jpg',
  'image/png': '.png',
  'text/plain': '.txt',
  'application/zip': '.zip',
  'application/x-rar-compressed': '.rar',   
  'application/x-tar': '.tar',             
  'application/x-7z-compressed': '.7z',    
  'video/mp4': '.mp4',
  'audio/mpeg': '.mp3',
};

// normalize volatile URLs (presigned params etc.) so skip-checks work reliably
function canonicalizeUrl(raw) {
  try {
    const u = new URL(raw);
    const volatile = new Set([
      'X-Amz-Signature','X-Amz-Expires','X-Amz-Credential','X-Amz-Date','X-Amz-Security-Token',
      'Expires','Signature','AWSAccessKeyId','response-content-disposition'
    ]);
    for (const k of [...u.searchParams.keys()]) {
      if (volatile.has(k)) u.searchParams.delete(k);
    }
    u.protocol = u.protocol.toLowerCase();
    u.hostname = u.hostname.toLowerCase();
    return u.toString();
  } catch {
    return raw;
  }
}

// Use onDeterminingFilename to capture download details and cancel
chrome.downloads.onDeterminingFilename.addListener((downloadItem, suggest) => {
  const downloadUrl = downloadItem.finalUrl || downloadItem.url;
  const canonUrl = canonicalizeUrl(downloadUrl);
  console.log(`[${new Date().toISOString()}] Primary capture via onDeterminingFilename: ID=${downloadItem.id}, URL=${downloadUrl}`);

  // Skip if already processed or marked safe
  if (processedDownloads.has(downloadItem.id) || safeDownloads.has(canonUrl)) {
    console.log(`[${new Date().toISOString()}] Download ID=${downloadItem.id} already processed or safe, letting it pass`);
    // do NOT return true unless you will call suggest() later
    if (suggest) {
      const name = downloadItem.filename?.split(/[\\/]/).pop() || getFallbackFilename(downloadUrl, downloadItem.mime);
      suggest({ filename: name, conflictAction: 'uniquify' }); // allow default save
    }
    return; //  not "return true"
  }

  // Immediately cancel to prevent saving to Downloads
  chrome.downloads.cancel(downloadItem.id, () => {
    if (chrome.runtime.lastError) {
      console.error(`[${new Date().toISOString()}] Failed to cancel download ID=${downloadItem.id}:`, chrome.runtime.lastError);
    } else {
      console.log(`[${new Date().toISOString()}] Download ID=${downloadItem.id} canceled in onDeterminingFilename`);
      // Erase partially downloaded file if it exists
      chrome.downloads.erase({ id: downloadItem.id }, () => {
        if (chrome.runtime.lastError) {
          console.error(`[${new Date().toISOString()}] Failed to erase download ID=${downloadItem.id}:`, chrome.runtime.lastError);
        } else {
          console.log(`[${new Date().toISOString()}] Download ID=${downloadItem.id} erased from download history`);
        }
      });
    }
  });

  // Mark as processed
  processedDownloads.add(downloadItem.id);

  // Try to get filename immediately
  let filename = downloadItem.filename;
  const nameOnly = filename ? filename.split(/[\\/]/).pop() : null;

  if (!nameOnly) {
    console.log(`[${new Date().toISOString()}] Filename missing in onDeterminingFilename for ID=${downloadItem.id}, polling...`);
    pollForFilename(downloadItem, (resolvedFilename) => {
      processDownload(downloadItem, resolvedFilename, suggest);
    });
  } else {
    console.log(`[${new Date().toISOString()}] Filename found: ${nameOnly}`);
    processDownload(downloadItem, filename, suggest);
  }

  // Return true to delay suggest only when we plan to suggest({cancel:true}) below
  return true;
});

// BACKUP: Use onCreated for downloads that bypass onDeterminingFilename
chrome.downloads.onCreated.addListener((downloadItem) => {
  const downloadUrl = downloadItem.finalUrl || downloadItem.url;
  const canonUrl = canonicalizeUrl(downloadUrl);

  // Skip if already processed or marked safe
  if (processedDownloads.has(downloadItem.id) || safeDownloads.has(canonUrl)) {
    console.log(`[${new Date().toISOString()}] Skipping already processed or safe download: ID=${downloadItem.id}, URL=${downloadUrl}`);
    return;
  }

  // Immediately cancel to prevent saving to Downloads
  chrome.downloads.cancel(downloadItem.id, () => {
    if (chrome.runtime.lastError) {
      console.error(`[${new Date().toISOString()}] Failed to cancel download ID=${downloadItem.id}:`, chrome.runtime.lastError);
    } else {
      console.log(`[${new Date().toISOString()}] Download ID=${downloadItem.id} canceled in onCreated`);
      // Erase partially downloaded file if it exists
      chrome.downloads.erase({ id: downloadItem.id }, () => {
        if (chrome.runtime.lastError) {
          console.error(`[${new Date().toISOString()}] Failed to erase download ID=${downloadItem.id}:`, chrome.runtime.lastError);
        } else {
          console.log(`[${new Date().toISOString()}] Download ID=${downloadItem.id} erased from download history`);
        }
      });
    }
  });

  console.log(`[${new Date().toISOString()}] Backup capture via onCreated: ID=${downloadItem.id}, URL=${downloadUrl}`);
  processedDownloads.add(downloadItem.id);

  // Try to get filename immediately
  let filename = downloadItem.filename;
  const nameOnly = filename ? filename.split(/[\\/]/).pop() : null;

  if (!nameOnly) {
    console.log(`[${new Date().toISOString()}] Filename missing in onCreated for ID=${downloadItem.id}, polling...`);
    pollForFilename(downloadItem, (resolvedFilename) => {
      processDownload(downloadItem, resolvedFilename, null);
    });
  } else {
    console.log(`[${new Date().toISOString()}] Filename found: ${nameOnly}`);
    processDownload(downloadItem, filename, null);
  }
});

// Poll for filename if not immediately available
function pollForFilename(downloadItem, callback) {
  const maxAttempts = 30; // Increased to 15s total (30 * 500ms)
  let attempts = 0;

  const poll = () => {
    attempts++;
    chrome.downloads.search({ id: downloadItem.id }, (results) => {
      if (results?.[0]?.filename) {
        console.log(`[${new Date().toISOString()}] Filename found after polling: ${results[0].filename}`);
        callback(results[0].filename);
      } else if (attempts < maxAttempts) {
        setTimeout(poll, 500);
      } else {
        console.warn(`[${new Date().toISOString()}] Filename polling timed out for ID=${downloadItem.id}`);
        const fallbackName = getFallbackFilename(downloadItem.url, downloadItem.mime);
        callback(fallbackName);
      }
    });
  };
  setTimeout(poll, 500);
}

// Process download: cancel, send to server, re-download if safe
function processDownload(downloadItem, filename, suggest) {
  const nameOnly = filename.split(/[\\/]/).pop() || getFallbackFilename(downloadItem.url, downloadItem.mime);
  const downloadUrl = downloadItem.finalUrl || downloadItem.url;
  const canonOriginal = canonicalizeUrl(downloadUrl);

  // Extract filename from URL's response-content-disposition if available
  let finalFilename = nameOnly;
  try {
    const urlObj = new URL(downloadUrl);
    const disposition = urlObj.searchParams.get('response-content-disposition');
    if (disposition) {
      const match = disposition.match(/filename="([^"]+)"/);
      if (match && match[1]) {
        finalFilename = match[1];
        console.log(`[${new Date().toISOString()}] Using filename from URL disposition: ${finalFilename}`);
      }
    }
  } catch (error) {
    console.error(`[${new Date().toISOString()}] Error parsing URL disposition:`, error);
  }
  console.log(`[${new Date().toISOString()}] Processing download: ID=${downloadItem.id}, filename=${finalFilename}, URL=${downloadUrl}`);

  // Send data to Go server for safety check with retry
  const attemptFetch = (retries = 3, delay = 1000) => {
    sendDataToGo(
      downloadItem.id,
      downloadUrl,
      finalFilename,
      downloadItem.mime
    )
      // once we get a response back then continue
      .then((response) => {
        console.log(`[${new Date().toISOString()}] Server response for ID=${downloadItem.id}:`, response);

        if (response.isSafe) {
          console.log(`[${new Date().toISOString()}] File is safe`);

          // Prefer local proxy served by Go (prevents single-use URL problems)
          const safeUrl = response.proxyUrl || downloadUrl;
          const canonSafe = canonicalizeUrl(safeUrl);

          // Mark URL as safe to avoid re-interception
          safeDownloads.add(canonSafe);

          // Retry download if URL might be expired
          const attemptDownload = (downloadRetries = 3) => {
            chrome.downloads.download(
              {
                url: safeUrl,
                filename: finalFilename,
                conflictAction: 'uniquify'
              },
              (newDownloadId) => {
                if (chrome.runtime.lastError) {
                  console.error(`[${new Date().toISOString()}] Failed to start new download for ID=${downloadItem.id}:`, chrome.runtime.lastError.message);
                  if (downloadRetries > 0) {
                    console.log(`[${new Date().toISOString()}] Retrying download (${downloadRetries} attempts left)...`);
                    setTimeout(() => attemptDownload(downloadRetries - 1), 1000);
                  } else {
                    safeDownloads.delete(canonSafe); // Clean up on failure
                    if (chrome.notifications && chrome.notifications.create) {
                      chrome.notifications.create({
                        type: 'basic',
                        iconUrl: 'icon.png',
                        title: 'Download Failed',
                        message: `Failed to start download for "${finalFilename}". Error: ${chrome.runtime.lastError.message}`
                      });
                    }
                  }
                } else {
                  console.log(`[${new Date().toISOString()}] New download started: ID=${newDownloadId}`);
                  processedDownloads.delete(downloadItem.id);
                  // Monitor download state
                  chrome.downloads.onChanged.addListener(function onChange(change) {
                    if (change.id === newDownloadId && change.state && (change.state.current === 'complete' || change.state.current === 'interrupted')) {
                      safeDownloads.delete(canonSafe);
                      console.log(`[${new Date().toISOString()}] Removed URL from safeDownloads: ${safeUrl}`);
                      chrome.downloads.onChanged.removeListener(onChange);
                    }
                  });
                }
              }
            );
          };
          attemptDownload();
        } else {
          console.warn(`[${new Date().toISOString()}] File is unsafe, download remains canceled: ID=${downloadItem.id}`);
          if (chrome.notifications && chrome.notifications.create) {
            chrome.notifications.create({
              type: 'basic',
              iconUrl: 'icon.png',
              title: 'Download Blocked',
              message: `The file "${finalFilename}" was blocked as it was deemed unsafe.`
            });
          }
        }
      })
      .catch((error) => {
        console.error(`[${new Date().toISOString()}] Error sending data for ID=${downloadItem.id}:`, error);
        if (retries > 0) {
          console.log(`[${new Date().toISOString()}] Retrying fetch (${retries} attempts left)...`);
          setTimeout(() => attemptFetch(retries - 1, delay * 2), delay);
        } else {
          console.error(`[${new Date().toISOString()}] Fetch retries exhausted for ID=${downloadItem.id}`);
          if (chrome.notifications && chrome.notifications.create) {
            chrome.notifications.create({
              type: 'basic',
              iconUrl: 'icon.png',
              title: 'Download Error',
              message: `Failed to verify safety of "${finalFilename}". Download canceled.`
            });
          }
        }
      });
  };

  attemptFetch(); // Start fetch with retries

  // If suggest exists, provide a filename and cancel
  if (suggest) {
    //  we are actively intercepting/canceling here
    suggest({ filename: finalFilename, conflictAction: 'uniquify', cancel: true });
  }
}

// Generate a fallback filename from URL and MIME type
function getFallbackFilename(url, mime) {
  try {
    const urlObj = new URL(url);
    let filename = urlObj.pathname.split('/').pop();
    const disposition = urlObj.searchParams.get('response-content-disposition');
    if (disposition) {
      const match = disposition.match(/filename="([^"]+)"/);
      if (match && match[1]) {
        return match[1];
      }
    }
    if (filename && filename.includes('.')) {
      return filename;
    }
    // Use MIME type to add extension
    const extension = mimeToExtension[mime] || '.bin';
    return `download_${Date.now()}${extension}`;
  } catch (error) {
    console.error(`[${new Date().toISOString()}] Error parsing URL for fallback filename:`, error);
    return `download_${Date.now()}${mimeToExtension[mime] || '.bin'}`;
  }
}

// Clean up processed downloads cache periodically
setInterval(() => {
  if (processedDownloads.size > 1000) {
    processedDownloads.clear();
    safeDownloads.clear();
    console.log(`[${new Date().toISOString()}] Cleared processed and safe downloads cache`);
  }
}, 10 * 60 * 1000);
