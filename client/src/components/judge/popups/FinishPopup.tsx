import Button from '../../Button';
import Popup from '../../Popup';
import Star from '../Star';
import TextArea from '../../TextArea';

interface FinishPopupProps {
    /* Function to modify the popup state variable */
    setEnabled: React.Dispatch<React.SetStateAction<boolean>>;

    /* Judge to vote on */
    judge: Judge;

    /* State variable for determining if popup is open */
    enabled: boolean;

    /* Callback function for flagging a project */
    callback: () => Promise<void>;

    // TODO: Export all this to a global store for the judge
    /* Starred status of project */
    starred: boolean;

    /* Setter function for starred status */
    setStarred: React.Dispatch<React.SetStateAction<boolean>>;

    /* Notes for project */
    notes: string;

    /* Setter function for notes */
    setNotes: React.Dispatch<React.SetStateAction<string>>;

    /* Opt-in challenges eligible for this project and each remaining quota */
    challenges: JudgeChallengeOptions;

    /* Selected challenge nominations */
    challengeStars: string[];

    /* Setter for selected challenge nominations */
    setChallengeStars: React.Dispatch<React.SetStateAction<string[]>>;
}

/**
 * Component to show when the user clicks the "Submit" button
 */
const FinishPopup = (props: FinishPopupProps) => {
    if (!props.enabled) return null;

    const done = async () => {
        await props.callback();
    };

    return (
        <Popup
            enabled={props.enabled}
            setEnabled={props.setEnabled}
            className="text-center max-h-[90vh] overflow-y-auto"
        >
            <h1 className="text-3xl font-bold text-primary">Judge Project</h1>
            <h2 className="text-xl font-bold">Finish judging this project</h2>
            <div className="flex flex-row justify-center text-left mt-2">
                <Star active={props.starred} setActive={props.setStarred} className="mr-4" />
                <p className="text-light">
                    Star projects you think should win the top places in the hackathon.
                </p>
            </div>
            <h3 className="text-lighter text-sm text-left mt-2">Personal Notes</h3>
            <TextArea
                label="Type any personal comments here"
                value={props.notes}
                setValue={props.setNotes}
                className="mt-1"
            />
            {props.challenges.challenges.length > 0 && (
                <div className="text-left mt-4">
                    <h3 className="font-bold">Opt-in challenges</h3>
                    <p className="text-sm text-light">
                        Nominate this project for a challenge it entered.
                    </p>
                    {props.challenges.challenges.map((challenge) => {
                        const remaining = props.challenges.remaining[challenge] ?? 0;
                        const selected = props.challengeStars.includes(challenge);
                        return (
                            <label key={challenge} className="flex items-center gap-2 py-2 text-sm">
                                <input
                                    type="checkbox"
                                    checked={selected}
                                    disabled={!selected && remaining === 0}
                                    onChange={() =>
                                        props.setChallengeStars((current) =>
                                            selected
                                                ? current.filter((name) => name !== challenge)
                                                : [...current, challenge]
                                        )
                                    }
                                />
                                <span>
                                    {challenge} ({remaining} left of {props.challenges.limit})
                                </span>
                            </label>
                        );
                    })}
                </div>
            )}
            <Button type="primary" onClick={done} className="mt-4">
                Submit
            </Button>
        </Popup>
    );
};

export default FinishPopup;
