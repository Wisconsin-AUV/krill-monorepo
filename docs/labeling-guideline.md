# Labeling guideline

How to draw boxes and assign attributes for each class. Every labeler reads this before starting.
Each section needs example images of correct and incorrect labels.

## General rules

- Boxes are tight around the visible part of the object.
- Rotate boxes of oriented types to the smallest rectangle around the object. A sign seen at an angle looks like a trapezoid, so the box covers its longest edges.
- Box objects even when they are only partly visible.
- One object is one track. Keep drawing the same track on later frames instead of starting a new one.
- Set track attributes (size, color, role) once. Tracks with unset attributes are left out of exports.
- If you can't tell which symbol an object shows, skip the frame without marking it done.
- Press Space only when every object in the frame has a box. Only frames marked done are exported.
- Press E for frames with no objects. Empty frames are exported as negative examples.
- Check the last frame of every tracked object. None of its boxes are exported until that frame is marked done.

## Keypoints

Types with keypoints still get a box around the whole object. Then place each keypoint, in the order the number and name by the cursor show.

- Place a keypoint where it is, even when something covers it or it's outside the frame, and mark it hidden (Alt+click, or O). Clear it (X) only when you can't tell where it is.
- A quad's corners go clockwise from the top left of the surface as it was printed, not as it appears on screen. A sheet turned on its side starts from the corner that was its top left.
- When the type has a reference design, it is laid over the quad. If the design looks turned or stretched, a corner is wrong.
- Shift+click a print inside the selected box to have SAM find its corners. Fix any keypoint SAM got wrong on one frame and press R. Tracking keeps the keypoints you placed on later frames.

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
- With keypoints, the sheet's four corners and the four outer corners of the crate's top rim. On angled views the rim corners go on the top of the near wall, not where the crate meets the floor.

## Table / Octagon

- `table`: the tabletop.
- `table_item`: each object on the table, one box per object.
- `basket`: the entire basket.
- `role_sign`: each image on the octagon wall.

## Gripper status

## Path markers

## Slalom
