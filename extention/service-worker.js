import {sendDataToGo} from './send_to_go.js';

chrome.downloads.onCreated.addListener(downloadItem => {
  console.log("New download started:");
  console.log("ID:", downloadItem.id);
  console.log("URL:", downloadItem.url);
  console.log("Filename:", downloadItem.filename);
  console.log("Mime type:", downloadItem.mime);

  chrome.downloads.cancel(downloadItem.id)

  //send the data to golang server via the function
  sendDataToGo(
    downloadItem.id,
    downloadItem.url,
    downloadItem.filename,
    downloadItem.mime
  );
});
