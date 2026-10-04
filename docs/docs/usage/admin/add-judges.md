---
sidebar_position: 3
title: Adding Judges
description: How to add judges to Jury
---

# Adding Judges

Just like accessing the [Add Projects](/docs/usage/admin/add-projects) page, go to the "Judges" tab on the admin dashboard and click the **Add Judges** button.

![Add Judges Page](./assets/add-judges.png)

## QR Code

Judge entry uses QR registration. On **Add Judges**, display the general judging QR code for your judging orientation. Each judge scans it, enters their name, and is signed in immediately. The welcome screen asks them to read the instructions and confirm the judging track. No email address or verification is needed.

![Judge QR Code](./assets/qr-code.png)

Check the admin dashboard to confirm judges are registered. Judges cannot receive projects until judging starts. Returning judges can reopen the judging portal in the same browser while their session is active; scanning a registration QR again also opens their existing session. Registration without an active session creates a new judge.

:::danger[Watch out!]
Keep in mind that the QR code will let **anyone add themselves to Jury**, so make sure stray hackers don't get the QR code! If your QR code gets leaked, you can use the **Generate new code** button on the QR code page to change the QR code. Note that anyone who has scanned the code but have yet to add themselves to the system will have to scan the new code.
:::

### Track QR Code

Track judges can also be added by their own QR code. When [track judging](/docs/usage/admin/tracks) is enabled, a dropdown will show up with each track that is listed. Select the track you would like add judges to and click the **[Track name] Track Judges** button.

![Track Judge QR Code](./assets/track-qr-code.png)

It will generate a different QR code for that track, which will be clearly noted by the title above the QR code.
