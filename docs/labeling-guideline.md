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

Types with keypoints still get a box around the whole object. Then click to place the keypoint whose number and name show by the cursor.

- Place a keypoint only where you can see it. Leave one that something covers or that's outside the frame unlabeled instead of guessing where it is, and clear one placed there (X).
- To skip one, press the number of the keypoint you'll place next (1 to 9), or step to it with W and Q.
- Once two of a quad's corners are placed, by you, tracking, or Fit keypoints, a click places the unplaced corner nearest it, whatever the cursor named before. Pressing a corner's number makes the next click place that one wherever it lands.
- A quad's corners go clockwise from the top left of the surface as it was printed, not as it appears on screen. A sheet turned on its side starts from the corner that was its top left.
- When the type has a reference design, it is laid over the quad. If the design looks turned or stretched, a corner is wrong.
- Shift+click a print inside the selected box to have SAM find its corners. Fix or clear any keypoint SAM got wrong on one frame and press R. Tracking keeps the keypoints you placed on later frames.

## Roles

Every type except `gate`, `torpedo_board`, and `table` has a `role` attribute for the symbol it shows. Export combines the type and role into one class, for example `role_sign-compass`.

## Gate

- `gate`: the entire frame.
- `role_sign`: the entire panel hanging from the gate.

## Torpedo

- `torpedo_board`: the outer edge of the board.
- `torpedo_hole`: a tight box on the hole opening.

## Bins

- `bin`: the whole bin, including the sheet inside it.
- Keypoints go on the sheet's four corners, not on the crate. A corner hidden by the crate's wall stays unlabeled. The center is placed for you only once all four corners are.

## Table / Octagon

- `table`: the tabletop.
- `table_item`: each object on the table, one box per object.
- `basket`: the entire basket.
- `role_sign`: each image on the octagon wall.

## Gripper status

## Path markers

## Slalom
