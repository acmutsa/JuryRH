---
sidebar_position: 1
title: Configuration
description: How to configure your Jury instance once set up
---

# Configuration and Settings

## Logging In

Once you have Jury up and running, make sure everything is working by logging into the admin portal. Use the admin password that you set in the environmental variables when setting up Jury. Once in the portal, you should see the admin dashboard shown below (though without any projects or judging progress):

![Admin Dashboard](./assets/dashboard.png)

Make sure where it shows “Test Hackathon Judging” on the picture is replaced by the name you set under `VITE_JURY_NAME` environmental variable.

On the dashboard, you should see a button at the top labeled **Settings**. This is where we will go to configure everything for Jury. Click it now.

## Settings

The settings page should look like the following:

![Admin Settings](./assets/admin-settings.png)

All settings will have a description describing their functionality, but we will go through each section one-by-one here for completeness. Note that a lot of the more destructive settings will be **disabled while judging is running**. This is to minimize the chance accidents happen and mess up judging.

### Judge Registration

Use **Disable Registration** to stop new judges joining through QR codes. Judges with active sessions can continue judging. **Max Registrations Per Minute** limits requests to `POST /qr/add` per IP address, using the existing `block_reqs` and `max_req_per_min` settings. Registration that is blocked or exceeds the limit returns HTTP 429.

### Judging Parameters

Here, we define a couple of the parameters for the judging process. The first two buttons **reassign project numbers** and **reassign judge groups** are pretty self-explanatory. The former will remove table numbers from all projects and re-assign table numbers. This is generally done in the order that projects are added to Jury. Make sure you do NOT do this during judging as it will mess up the judging order! Generally, we use this if we've deleted a lot of projects in Jury and want to fill in the "holes" in tables (eg. you remove table 46 and want to shift all the other tables to fill the gap--this will obviously be bad during judging as it'll move all later tables). If you want to do this during judging (eg. add a new project to an empty table), you can use the "set table number" action on the [dashboard](/docs/usage/admin/dashboard). The **reassign judge groups** is a little complicated and will be explained on the [multi-group judging page](/docs/usage/admin/groups).

**Set Minimum Project Views** allows for you to specify the minimum number of views a project should get. The goal of Jury is to ensure that every project is compared to every other project approximately an equal number of times. However, we want to ensure all projects do get seen at LEAST a couple of times. Until all projects have been seen n times (set by this setting), the projects with the least number of views will be given to the next judge. This ensures that we are prioritizing all low-view projects until passing this threshold. Once past the threshold, the comparison-balancing algorithm will then be used to assign projects to judges.

**Ignore Tracks** is a feature primarily for batch uploading projects. If you have a track that you do NOT want judged with Jury, you can insert tracks that you want Jury to ignore here. When project(s) are uploaded to Jury, any project containing a track in the "Ignore Tracks" list will not be added to Jury. If you want all projects to be added to Jury, this is NOT where tracks to be judged are defined (see [below](#multi-group-and-track-judging)).

### Judging Clock and Timer

**Reset Main Clock** will reset the clock on the admin page--this is handy if you accidentally start the main clock and want to quickly reset it.

**Sync Clock with Database Automatically** is only useful if you have multiple instances of Jury or experience crashes. This will periodically store the main clock time in the database so that if Jury happens to restart the time won't be reset (as it's normally stored in memory).

**Backup Clock** will store the current clock value in the database.

**Set Judging Timer** determines how long each judge has to see a project. Judges will see a timer on their phone for each project, with its length determined by the value you set here. We find that 5 minutes is generally a good length and is defaulted to that.

### Multi-Group and Track Judging

In this section, you can toggle the multi-group judging and track judging options. Track judging allows judges to view only projects from specific tracks and judge them separately from the main judging. Multi-group judging can be used to group judges so they will only be assigned projects in their group, switching groups periodically to minimize walking. More detailed information about how to use track and group judging are on their respective pages:

- [Track Judging](/docs/usage/admin/tracks)
- [Multi-Group Judging](/docs/usage/admin/groups)

### Export Data

This section allows you to export data from Jury as a CSV. There are 4 groupings to export:

- **Export Judges** - Exports all judges
- **Export Projects** - Exports all projects
- **Export Rankings** - Exports only the judges and their individual rankings of projects
- **Export by Challenges** - Exports a zip file, with each CSV file containing only the projects that submitted to a specific challenge; each CSV will be titled with that challenge name

Project CSVs put judging results immediately after Name and Table, before descriptions and links. They include live **Score** and **Stars** totals from general judging, **Track Score**, **Track Stars**, and **Track Seen** columns for each configured track, and **Challenge Stars** for each challenge enabled in Settings. The per-challenge ZIP contains the same columns, filtered to projects entered in that challenge. Scores use the same Copeland calculation as the dashboard; challenge nominations are separate from general and track stars. A zero means no recorded total for an eligible project; a blank means the project did not enter that track or challenge.

### Reset Data

At the bottom of the settings page, there are multiple ways to reset data in Jury:

- **Clear Rankings and Stars**: This will remove all rankings that judges have made but keep all other info (judges, projects, settings).
- **Clear Judging Data**: On top of removing scores, this will also remove all views and judging progress, resetting Jury to right before judging started.
- **Delete all Projects**: This will delete all projects, judging data, and flags. It will not delete settings and judges.
- **Delete all Judges**: This will delete all judges, judging data, and flags. It will not delete settings and projects.
- **Drop Database**: This button will reset the database. It's pretty destructive, so obviously do not click it unless you are sure you want to COMPLETELY CLEAR Jury!!!

## Challenge Stars

Open **Settings → Challenge Stars** to enable star nominations for individual opt-in challenges. Challenges appear after projects with challenge entries are added. General judges see star buttons in the finish dialog only for enabled challenges that the current project entered. Each judge has a separate nomination quota per challenge (two by default). This section also shows the projects nominated and their challenge star totals; use **Refresh challenge stars** to update them.

Track judges use ordered rankings and optional track stars. Challenge nominations are available during general judging.

During judging, **Done → Challenge Stars** shows nominations above personal notes. The dialog refreshes eligibility when opened. It shows a reason if stars are unavailable: the judge is assigned to a track, no challenges are enabled, or the project has not entered an enabled challenge. A failed request shows a retry button. Exhausted quotas keep the challenge visible with its star disabled.

Exports fetch fresh judging results. If **Export Projects** reports an older export format, update/restart the backend serving the frontend's configured `VITE_JURY_URL`, then export again. The expected project CSV includes `Score` and `Stars`, plus the configured track and enabled challenge result columns.
