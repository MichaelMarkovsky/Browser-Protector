import {sendDataToGo} from './send_to_go.js';

chrome.downloads.onCreated.addListener(downloadItem => {
  console.log("New download started:");
  console.log("ID:", downloadItem.id);
  console.log("URL:", downloadItem.url);
  console.log("Filename:", downloadItem.filename);
  console.log("Mime type:", downloadItem.mime);

  console.log('Download paused:',downloadItem.id)
  chrome.downloads.pause(downloadItem.id)

   // wait for the filename to be available
  // wait for the filename to be available
  const listener = delta => {
    if (delta.id === downloadItem.id && delta.filename && delta.filename.current) {
      const nameOnly = delta.filename.current.split(/[\\/]/).pop();
      console.log("Final filename:", nameOnly);

      // send the data to golang server
      sendDataToGo(
        downloadItem.id,
        downloadItem.url,
        nameOnly,
        downloadItem.mime
      );

      // stop listening for this download
      chrome.downloads.onChanged.removeListener(listener);
    }
  };
  chrome.downloads.onChanged.addListener(listener);
});