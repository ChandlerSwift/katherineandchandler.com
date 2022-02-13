#!/usr/bin/python

import sqlite3
import csv
con = sqlite3.connect("attendees.db")
cur = con.cursor()

parties = []

with open("guestlist.csv") as csvfile:
    reader = csv.DictReader(csvfile)
    for row in reader:
        if not row["Party"] in parties:
            parties.append(row["Party"])
            cur.execute("INSERT INTO parties (name) values (?)", (row["Party"],))
        cur.execute(
            "INSERT INTO attendees (name, invited_to_rehearsal_dinner, party_id) values (?, ?, (select id from parties where name = ?))",
            (row["Name "], row["Rehearsal Dinner"].lower() == "yes", row["Party"]),
        )
con.commit()
