# Labeling guideline

How to draw boxes and assign attributes for each class. Every labeler reads this before starting.
Each section needs example images of correct and incorrect labels.

## General rules

- Boxes are tight around the visible part of the object.
- Box objects even when they are only partly visible.
- One object is one track. Keep drawing the same track on later frames instead of starting a new one.
- Set track attributes (size, color, role) once. Tracks with unset attributes are left out of exports.
- If you can't tell which symbol an object shows, skip the frame without marking it done.
- Press Space only when every object in the frame has a box. Only frames marked done are exported.
- Press E for frames with no objects. Empty frames are exported as negative examples.

## Roles

Every type except `gate`, `torpedo_board`, and `table` has a `role` attribute for the symbol it shows. Export combines the type and role into one class, for example `role_sign-compass`.

## Gate

- `gate`: the entire frame.
- `role_sign`: the entire panel hanging from the gate.

## Torpedo

- `torpedo_board`: the outer edge of the board.
- `torpedo_hole`: a tight box on the hole opening.

## Bins

- `bin`: the whole bin opening, including the image inside it.

## Table / Octagon

- `table`: the tabletop.
- `table_item`: each object on the table, one box per object.
- `basket`: the entire basket.
- `role_sign`: each image on the octagon wall.

## Gripper status

## Path markers

## Slalom
