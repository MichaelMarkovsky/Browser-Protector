chrome.downloads.onCreated.addListener(downloadItem => {
  console.log("New download started:");
  console.log("ID:", downloadItem.id);
  console.log("URL:", downloadItem.url);
  console.log("Filename:", downloadItem.filename);
  console.log("Mime type:", downloadItem.mime);
});
